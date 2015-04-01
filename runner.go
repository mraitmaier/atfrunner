package main

/*
 * runner.go - the definition of the Runner struct & type, the main GoATF
 * CLI application data structure
 */
import (
	"bitbucket.org/miranr/atf"
	"bitbucket.org/miranr/atf/utils"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
)

// Runner is structure that holds all data needed for running the application.
type Runner struct {
	tr      *atf.TestReport // TestSet that's be run
	input   string          // input configuration file (currently only JSON)
	workdir string          // working directory
	logfile string
	syslog  string
	report  string
	cssfile string
	xml     bool       // create XML report (beside HTML report)
	json    bool       // create JSON report (beside HTML report)
	par     bool       // run tests in parallel? (default: false) TODO
	debug   bool       // enable debug mode (for testing purposes only)
	logger  *utils.Log // a logger instance
}

// NewRunner creates new Runner instance and return its pointer.
func NewRunner() *Runner {

	var r = new(Runner)
	r.logger = utils.NewLog()
	r.par = false // run sequentially by default
	return r
}

// Displays the contents of the Runner type. If complete flag is 'true', method will display the complete TestSet;
// otherwise only name will be printed
func (r *Runner) display(complete bool) {

	fmt.Printf("Input config file: %q\n", r.input)
	fmt.Printf("Working dir: %q\n", r.workdir)
	fmt.Printf("Log filename: %q\n", r.logfile)
	fmt.Printf("Syslog server IP: %q\n", r.syslog)
	fmt.Printf("Final report name: %q\n", r.report)
	fmt.Printf("(Optional) CCS file for HTML report: %q\n", r.cssfile)
	fmt.Printf("Debug node enabled? %t\n", r.debug)
	fmt.Printf("Parallel execution? %t\n", r.par)

	// display loggers
	fmt.Printf("Loggers:\n")
	if r.logger != nil {
		fmt.Println(r.logger.String())
	}

	// display test set
	if r.tr != nil {
		if complete {
			fmt.Printf("%s", r.tr.String())
		} else {
			fmt.Printf("TestSet: %q\n", r.tr.Name())
		}
	} else {
		fmt.Println("TestSet not defined yet.")
	}
}

// setWorkDir - set the working directory Join both of the input parameters into proper system PATH.
// If both input parameters are empty strings, create the default value; this is OS dependant: on WinXY the default is bound
// to USERPROFILE environment variable, while on POSIX systems, the default is bound to HOME envronment variable.
func (r *Runner) setWorkDir(basedir string, tsName string) {
	if basedir == "" {
		if runtime.GOOS == "windows" {
			basedir = os.Getenv("USERPROFILE")
		} else {
			basedir = os.Getenv("HOME")
		}
		basedir = path.Join(basedir, "atfrunner",
			fmt.Sprintf("%s_%s", tsName, utils.NowFile()))
	}
	r.workdir = filepath.ToSlash(basedir)
}

// Runner.collect - collect the configuration that'll be executed Parse the configuration file and create/update the appropriate
// data structures - first of all the TestSet.
func (r *Runner) collect() (err error) {

	ts := new(atf.TestSet)
	//ts.Sut = new(atf.SysUnderTest)

	if r.input != "" {
		ts = atf.Collect(r.input)
	} else {
		return errors.New("There's no configuration file defined.")
	}

	if ts == nil {
		return errors.New("Test set is empty.")
	}
	r.tr = atf.CreateTestReport(ts)
	return
}

// Let's define the default levels for different log handlers: all text goes only to file logger, console should take only the most
// important printous, while syslog handler should omit sending the execution outputs.
const (
	defSyslogLevel utils.Severity = utils.Notice
	defFileLevel   utils.Severity = utils.Informational
	defStreamLevel utils.Severity = utils.Notice
)

// the max number of loggers used here (console, file & syslog)
//const numOfLoggers int = 3

// Creates all needed log handlers.
func (r *Runner) createLog() error {

	logfile := ""
	// logfile input argument is NOT empty...
	if r.logfile != "" {
		// and represents absolute path, take it as it is.
		if path.IsAbs(r.logfile) {
			logfile = r.logfile
		} else {
			// if not absolute path, get working dir and join the path to filename
			logfile = path.Join(r.workdir, r.logfile)
		}
	} else {
		logfile = path.Join(r.workdir, "output.log")
	}
	r.logfile = logfile
	// now the real thing...
	format := "%s %s %s"
	err := r.createLoggers(format, r.debug)
	if err != nil {
		return err
	}
	r.logger.Start()

	// if logger is created, this message should print...
	r.logger.Warning("Log successfully created\n")
	return nil
}

// this function actually creates all the log handlers.
func (r *Runner) createLoggers(format string, debug bool) error {
	// first, we define log levels (severity)
	fLevel := defFileLevel   // this is level for file handler
	sLevel := defSyslogLevel // this is level for syslog & console handlers
	if debug {
		fLevel = utils.Debug
		sLevel = utils.Debug
	}
	// now create file logger
	f, err := utils.NewFileHandler(r.logfile, format, fLevel)
	if err != nil {
		return err
	}
	if f != nil {
		r.logger.Handlers = r.logger.AddHandler(f)
	}
	// and create console logger
	l := utils.NewStreamHandler(format, sLevel)
	if l != nil {
		r.logger.Handlers = r.logger.AddHandler(l)
	}
	// and finally create syslog logger if needed
	if r.syslog != "" {
		var s *utils.SyslogHandler
		s = utils.NewSyslogHandler(r.syslog, format, sLevel)
		if s != nil {
			r.logger.Handlers = r.logger.AddHandler(s)
		}
	}
	return err
}

// Initializes the Runner instance.
func (r *Runner) initialize() error {
	// let's collect the configuration
	err := r.collect()
	if err != nil {
		return err
	}
	// check working dir value; if empty, redefine to default: '$HOME/results'
	r.setWorkDir(r.workdir, r.tr.TestSet.Name)
	// if this dir is not existent, create it
	err = os.MkdirAll(r.workdir, 0755)
	if err != nil {
		return err
	}
	// create log file
	err = r.createLog()
	return err
}

// Run starts the Runner instance; it executes the test sequence.
func (r *Runner) Run() {
	// define a logging closure to be passed around...
	fn := atf.ExecDisplayFnCback(func(params ...string) {
		// we check that at least two string args are present; if more, we
		// ignore them
		if len(params) < 2 {
			panic("Callback: Wrong number of parameters.")
		}
		// now log the message
		lvl := params[0] // the first arg is logging level
		msg := params[1] // the second arg is logging message
		r.logger.LogS(lvl, msg)
	})

	// execution begins...
	r.tr.Started = utils.Now()
	r.logger.Notice(fmt.Sprintf("     Started: %s\n", r.tr.Started))

	// run test set only if it's not empty...
	if r.tr.TestSet != nil {
		r.logger.Notice(fmt.Sprintf("# Starting Test set: %q\n",
			r.tr.TestSet.Name))
		r.tr.TestSet.Execute(&fn) // we pass a ptr to defined closure
	}

	r.tr.Finished = utils.Now()
	r.logger.Notice(fmt.Sprintf("# Test set: %q end.\n", r.tr.TestSet.Name))
	r.logger.Notice(fmt.Sprintf("     Finished: %s\n", r.tr.Finished))
	// This is the end of execution

}

// The createHtmlHeader function creates the header of the execution HTML report.
const mandatoryCSS = "cfg/always.css"

func (r *Runner) createHTMLHeader(name string) string {
	s := "<!DOCTYPE html>\n"
	s += "<html>\n<head>\n"
	s += fmt.Sprintf("<meta charset=%q>\n", "utf-8")
	s += fmt.Sprintf("<title>Report: %s</title>\n", name)
	// include CSS file; default CSS is "cfg/report_def.css"
	s += "<link rel=\"stylesheet\" type=\"text/css\" "
	_, f1 := path.Split(mandatoryCSS)
	s += fmt.Sprintf("href=%q>\n", f1)
	s += "<link rel=\"stylesheet\" type=\"text/css\" "
	_, f2 := path.Split(r.cssfile)
	s += fmt.Sprintf("href=%q>\n", f2)
	s += "</head>\n"
	return s
}

// The createXMLHeader function creates the execution XML report.
func (r *Runner) createXMLReport(filename string) error {
	x := fmt.Sprintf("<?xml version=%q encoding=%q?>", "1.0", "UTF-8")
	// create XML representation
	trXML, err := r.tr.XML()
	if err != nil {
		return err
	}
	x += trXML

	// write XML file
	fout, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer fout.Close()
	fmt.Fprint(fout, x)
	return nil
}

// The createJSONHeader function creates the execution JSON report.
func (r *Runner) createJSONReport(filename string) error {

	json, err := r.tr.JSON()
	if err != nil {
		return err
	}

	//
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprint(f, json)
	return nil
}

// The createHTMLReport function creates the execution HTML report.
func (r *Runner) createHTMLReport(filename string) error {
	// HTML report is always created
	html := r.createHTMLHeader(r.tr.TestSet.Name)
	html += "<body>\n"
	h, err := r.tr.HTML()
	if err != nil {
		return err
	}
	html += h
	html += "</body>\n</html>\n"
	// the file itself
	fout, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer fout.Close()
	fmt.Fprint(fout, html)
	// copy the CSS files with HTML report
	_, f1 := path.Split(mandatoryCSS)
	_, f2 := path.Split(r.cssfile)
	_, err = utils.CopyFile(path.Join(r.workdir, f1), mandatoryCSS)
	_, err = utils.CopyFile(path.Join(r.workdir, f2), r.cssfile)
	if err != nil {
		return err
	}
	return nil
}

// CreateReports creates the configured reports after the execution is over.
func (r *Runner) CreateReports() {
	// always create HTML report
	filename := filepath.ToSlash(path.Join(r.workdir, "report.html"))
	err := r.createHTMLReport(filename)
	if err != nil {
		r.logger.Error("XML report could not be created.\n")
		r.logger.Error(fmt.Sprintf("Reason: %s\n", err))
		return
	}
	r.logger.Notice(fmt.Sprintf("HTML report %q created.\n", filename))

	// create XML report, if needed
	if r.xml {
		filename = filepath.ToSlash(path.Join(r.workdir, "report.xml"))
		err := r.createXMLReport(filename)
		if err != nil {
			r.logger.Error("XML report could not be created.\n")
			r.logger.Error(fmt.Sprintf("Reason: %s\n", err))
			return
		}
		r.logger.Notice(fmt.Sprintf("XML report %q created.\n", filename))
	}

	// JSON report upon request
	if r.json {
		filename = filepath.ToSlash(path.Join(r.workdir, "report.json"))
		err := r.createJSONReport(filename)
		if err != nil {
			r.logger.Error("JSON report could not be created.\n")
			r.logger.Error(fmt.Sprintf("Reason: %s\n", err))
			return
		}
		r.logger.Notice(fmt.Sprintf("JSON report %q created.\n", filename))
	}
}

// SetParallel sets the flag to execute the test cases in parallel.
func (r *Runner) SetParallel() { r.par = true }
