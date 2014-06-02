/*
 * main.go - the file to rule them all
 */
package main

import (
	"flag"
	"fmt"
	"os"
)

/************************************************
 * parseArgs - parge command-line arguments
 */
func parseArgs(r *Runner) {
	flag.StringVar(&r.input, "i", "", "Input configuration path")
	flag.StringVar(&r.workdir, "w", "", "Working directory path")
	flag.StringVar(&r.logfile, "l", "", "Logfile name")
	flag.StringVar(&r.syslog, "s", "", "Syslog server IP")
	flag.StringVar(&r.report, "r", "", "final report filename")
	flag.StringVar(&r.cssfile, "c", "cfg/report_def.css",
		"custom CSS file for HTML report")
	flag.BoolVar(&r.xml, "X", false, "create XML report (beside HTML report)")
	flag.BoolVar(&r.json, "J", false, "create JSON report (beside HTML report)")
	flag.BoolVar(&r.debug, "d", false,
		"enable debug mode (for testing purposes)")
	//
	flag.Parse()
}

/*
 * main -
 */
func main() {
//	    atf.RunBats() // for testing purposes : test/bats.go
	r := NewRunner()
	// parse CLI arguments
	parseArgs(r)
	// initialize new Runner; if initializaton fails, exit gracefully 
	err := r.initialize()
	if err != nil {
		fmt.Println(err)
		fmt.Println("Please define the input configuration file")
		fmt.Println("Use '-h' switch to display help")
		fmt.Println("Exiting...")
		os.Exit(1)
	}
//	r.display(true) // DEBUG
	// now, run the damn thing....
	r.Run()
	//
	//r.display(true) // DEBUG
	r.CreateReports()
	// close the logger
	r.logger.Close()
}
