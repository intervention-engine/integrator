HIE Integrator [![Build Status](https://travis-ci.org/intervention-engine/integrator.svg?branch=master)](https://travis-ci.org/intervention-engine/integrator)
=================================================================================================================================================================================================

The *integrator* project provides a tool for integrating data from an HIE into the [Intervention Engine](https://github.com/intervention-engine/ie). The integrator tool queries an HIE for CDA documents and then posts them to IE's [CCDA Endpoint](https://github.com/intervention-engine/ie-ccda-endpoint).   The integrator also keeps track of which documents have already been transferred in order to avoid transfering the same data twice.

The integrator tool is a simple commandline utility that is intended to be run as a cron job -- allowing data to be batch imported on a regular basis.  It is currently designed for a specific use case and endpoint, but future versions may be more configurable to support other use cases and endpoints. 

Building and Running integrator Locally
---------------------------------------------------

Intervention Engine is a stack of tools and technologies. For information on installing and running the full stack, please see [Building and Running the Intervention Engine Stack in a Development Environment](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md).

Please refer to the following sections in the above guide to install the necessary dependencies (Git and Go):

-	(Prerequisite) [Install Git](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md#install-git)
-	(Prerequisite) [Install Go](https://github.com/intervention-engine/ie/blob/master/docs/dev_install.md#install-go)

Remainder of documentation to be continued...

License
-------

Copyright 2016 The MITRE Corporation

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.
