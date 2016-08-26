package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/robfig/cron"

	"gopkg.in/mgo.v2"
)

func main() {
	hieFlag := flag.String("hie", "", "HIE API Endpoint URL (env: HIE_URL)")
	userFlag := flag.String("user", "", "User account name to use for authentication to HIE (env: HIE_USER)")
	passwordFlag := flag.String("password", "", "Password to use for authentication to HIE (env: HIE_PASSWORD)")
	ingestFlag := flag.String("ingest", "", "Ingest API Endpoint URL (env: INGEST_URL)")
	eeFlag := flag.String("ee", "", "EE number to copy data for (env: EE).  User must supply 'ee' OR 'eeFile'.")
	eeFileFlag := flag.String("eeFile", "", "Path to a file with an EE number on each line (env: EE_FILE).  User must supply 'ee' OR 'eeFile'.")
	formatsFlag := flag.String("formats", "", "Comma-separate list of supported document formats (env: FORMATS, default: \"XML^HL7^231^CCD^C32,XML^HL7^231^CCD^V1.1\")")
	curlFlag := flag.Bool("curl", false, "Flag to indicate if system CUrl command should be used (env: USE_CURL, default: false).  Only use if go http lib isn't working (e.g., tls renegotiation)")
	mongoFlag := flag.String("mongo", "", "MongoDB address (env: MONGO_URL, default: \"mongodb://localhost:27017\")")
	copyDirFlag := flag.String("copy-dir", "", "Path to a folder where HIE records should be copied locally (env: COPY_DIR, default: none)")
	cronFlag := flag.String("cron", "", "Cron expression indicating when the integrator tool should run to refresh data (env: INTEGRATOR_CRON, example: \"0 0 20 * * *\").  If cron is not supplied, \"now\" must be set.")
	nowFlag := flag.Bool("now", false, "Flag to indicate if the integrator should run immediately (env: INTEGRATOR_NOW, default: false).  If used without cron, integrator will run once and then exit.  If now is not set, \"cron\" must be supplied.")
	flag.Parse()

	hie := getRequiredConfigValue(hieFlag, "HIE_URL", "HIE URL")
	user := getConfigValue(userFlag, "HIE_USER", "")
	password := getConfigValue(passwordFlag, "HIE_PASSWORD", "")
	ingest := getRequiredConfigValue(ingestFlag, "INGEST_URL", "Ingest URL")
	if strings.HasPrefix(ingest, ":") {
		ingest = "http://localhost" + ingest
	}

	ee := getConfigValue(eeFlag, "EE", "")
	eeFile := getConfigValue(eeFileFlag, "EE_FILE", "")
	if ee == "" && eeFile == "" {
		fmt.Fprintln(os.Stderr, "EE or EE File must be passed in as an argument or environment variable.")
		flag.PrintDefaults()
		os.Exit(1)
	}
	var err error
	var eeSlice []string
	if ee != "" {
		eeSlice = []string{ee}
	} else {
		eeSlice, err = parseEEFile(eeFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Couldn't get EE numbers from ee file:", err.Error())
			os.Exit(1)
		}
	}

	formats := getConfigValue(formatsFlag, "FORMATS", "XML^HL7^231^CCD^C32,XML^HL7^231^CCD^V1.1")
	fmtSlice := strings.Split(formats, ",")

	curl := getBoolConfigValue(curlFlag, "USE_CURL")
	mongo := getConfigValue(mongoFlag, "MONGO_URL", "mongodb://localhost:27017")
	if strings.HasPrefix(mongo, ":") {
		mongo = "mongodb://localhost" + mongo
	}
	copyDir := getConfigValue(copyDirFlag, "COPY_DIR", "")

	cronSpec := getConfigValue(cronFlag, "INTEGRATOR_CRON", "")
	now := getBoolConfigValue(nowFlag, "INTEGRATOR_NOW")
	if cronSpec == "" && !now {
		fmt.Fprintln(os.Stderr, "Cron and/or the now flag must be specified")
		flag.PrintDefaults()
		os.Exit(1)
	}

	session, err := mgo.Dial(mongo)
	if err != nil {
		panic("Can't connect to the database")
	}
	defer session.Close()
	db := session.DB("integrator")

	txLogManager, err := NewMgoTransactionLogManager(db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error configuring the log manager:", err.Error())
		os.Exit(1)
	}

	var hieClient *HttpHieClient
	if user != "" {
		if curl {
			hieClient = NewCUrlBasicAuthHttpHieClient(hie, user, password)
		} else {
			hieClient = NewBasicAuthHttpHieClient(hie, user, password)
		}
	} else {
		if curl {
			hieClient = NewCUrlHttpHieClient(hie)
		} else {
			hieClient = NewHttpHieClient(hie)
		}
	}

	ingestClient := NewHttpIngestClient(ingest)

	var dataCopier *DataCopier
	if copyDir == "" {
		dataCopier, err = NewDataCopier(hieClient, ingestClient, txLogManager)
	} else {
		dataCopier, err = NewDataCopierWithLocalCopies(hieClient, ingestClient, txLogManager, copyDir)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error configuring the data copier:", err.Error())
		os.Exit(1)
	}

	copyFn := func() {
		for _, eeNum := range eeSlice {
			err := dataCopier.CopyRecords(eeNum, fmtSlice...)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error copying data for ee %s: %s\n", eeNum, err.Error())
			}
		}
	}

	if now {
		copyFn()
	}

	if cronSpec != "" {
		c := cron.New()
		err = c.AddFunc(cronSpec, copyFn)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Can't setup cron job for integrator. Specified spec:", cronSpec)
			os.Exit(1)
		}
		c.Start()
		defer c.Stop()
		select {} // this causes the program to block rather than just exit right away
	}
}

func parseEEFile(eeFile string) ([]string, error) {
	f, err := os.Open(eeFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var ees []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			ees = append(ees, line)
		}
	}
	return ees, scanner.Err()
}

func getConfigValue(parsedFlag *string, envVar string, defaultVal string) string {
	val := *parsedFlag
	if val == "" {
		val = os.Getenv(envVar)
		if val == "" {
			val = defaultVal
		}
	}
	return val
}

func getBoolConfigValue(parsedFlag *bool, envVar string) bool {
	val := *parsedFlag
	if !val && os.Getenv(envVar) != "" {
		switch os.Getenv(envVar) {
		case "true", "TRUE", "YES", "yes", "ON", "on", "1":
			val = true
		case "false", "FALSE", "NO", "no", "OFF", "off", "0":
			val = false
		default:
			fmt.Fprintf(os.Stderr, "%s is not a valid value for a boolean.\n", os.Getenv(envVar))
			flag.PrintDefaults()
			os.Exit(1)
		}
	}
	return val
}

func getRequiredConfigValue(parsedFlag *string, envVar string, name string) string {
	val := getConfigValue(parsedFlag, envVar, "")
	if val == "" {
		fmt.Fprintf(os.Stderr, "%s must be passed in as an argument or environment variable.\n", name)
		flag.PrintDefaults()
		os.Exit(1)
	}
	return val
}
