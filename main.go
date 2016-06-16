package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"gopkg.in/mgo.v2"
)

func main() {
	hieAddr := flag.String("hie", "", "HIE API Endpoint URL")
	user := flag.String("user", "", "User account name to use for authentication to HIE")
	password := flag.String("password", "", "Password to use for authentication to HIE")
	ingestAddr := flag.String("ingest", "", "Ingest API Endpoint URL")
	ee := flag.String("ee", "", "EE number to copy data for (user must supply 'ee' OR 'eeFile')")
	eeFile := flag.String("eeFile", "", "Path to a file with an EE number on each line (user must supply 'ee' OR 'eeFile')")
	formats := flag.String("formats", "", "Comma-separate list of supported document formats")
	mongoAddr := flag.String("mongo", "", "MongoDB address (default: \"mongodb://localhost:27017\")")
	flag.Parse()

	if *hieAddr == "" || *ingestAddr == "" || (*ee == "" && *eeFile == "") {
		flag.PrintDefaults()
	}

	var err error
	var eeSlice []string
	if *ee != "" {
		eeSlice = []string{*ee}
	} else {
		eeSlice, err = parseEEFile(*eeFile)
		if err != nil {
			fmt.Println("Couldn't get EE numbers from ee file:", err.Error())
			os.Exit(1)
		}
	}

	fmtSlice := strings.Split(*formats, ",")

	// Prefer mongo arg, falling back to env, falling back to default
	mongo := *mongoAddr
	if mongo == "" {
		mongo := os.Getenv("MONGO_PORT_27017_TCP_ADDR")
		if mongo == "" {
			mongo = "mongodb://localhost:27017"
		}
	} else if strings.HasPrefix(mongo, ":") {
		mongo = "mongodb://localhost" + mongo
	}
	session, err := mgo.Dial(mongo)
	if err != nil {
		panic("Can't connect to the database")
	}
	defer session.Close()
	db := session.DB("integrator")

	txLogManager, err := NewMgoTransactionLogManager(db)
	if err != nil {
		fmt.Println("Error configuring the log manager:", err.Error())
		os.Exit(1)
	}

	var hieClient *HttpHieClient
	if *user != "" {
		hieClient = NewBasicAuthHttpHieClient(*hieAddr, *user, *password)
	} else {
		hieClient = NewHttpHieClient(*hieAddr)
	}

	ingestClient := NewHttpIngestClient(*ingestAddr)

	dataCopier, err := NewDataCopier(hieClient, ingestClient, txLogManager)
	if err != nil {
		fmt.Println("Error configuring the data copier:", err.Error())
		os.Exit(1)
	}

	for _, eeNum := range eeSlice {
		err := dataCopier.CopyRecords(eeNum, fmtSlice...)
		if err != nil {
			fmt.Printf("Error copying data for ee %s: %s\n", eeNum, err.Error())
		}
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
