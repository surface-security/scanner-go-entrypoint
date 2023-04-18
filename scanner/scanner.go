package scanner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	flag "github.com/spf13/pflag"
)

// Options holds the default options for a scanner
type Options struct {
	Input      string   // input file
	Output     string   // output directory
	BinPath    string   // path to the scanner executable
	ExtraHelp  bool     // show scanner executable help
	ExtraFlags []string // extra flags passed directly to the scanner - use ExtraHelp to show them
}

// Scanner holds scanner configuration
type Scanner struct {
	Name          string   // name of the scanner
	DefaultBinary string   // default value for BinPath - defaults to `Name`
	Options       *Options // holds the CLI options
}

func (s *Scanner) GetDefaultBinaryPath() string {
	if s.DefaultBinary != "" {
		return s.DefaultBinary
	}
	return s.Name
}

// BuildOptions parses the command line flags provided by a user
func (s *Scanner) BuildOptions() *Options {
	s.Options = &Options{}
	flag.StringVarP(&s.Options.Output, "output", "o", "/output", "Scanner results directory")
	flag.StringVarP(&s.Options.BinPath, "bin", "b", s.GetDefaultBinaryPath(), "Path to scanner binary")
	flag.BoolVarP(&s.Options.ExtraHelp, "scanner-help", "H", false, "Show help for the scanner extra flags")
	return s.Options
}

// Exec executes scanner binary with given args plus all extra flags
func (s *Scanner) Exec(args ...string) error {
	flags := append(
		args,
		s.Options.ExtraFlags...,
	)
	cmd := exec.Command(s.Options.BinPath, flags...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ExecCaptureOutput executes scanner binary with given args plus all extra flags and passes stdout line by line to callback
func (s *Scanner) ExecCaptureOutput(callback func(string), args ...string) error {
	flags := append(
		args,
		s.Options.ExtraFlags...,
	)
	cmd := exec.Command(s.Options.BinPath, flags...)
	r, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr

	done := make(chan struct{})
	scanner := bufio.NewScanner(r)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			callback(line)
		}
		done <- struct{}{}
	}()
	err = cmd.Run()
	// Wait for all output to be processed
	<-done
	return err
}

func ReadInputLines(options *Options, callback func(string) bool) {
	readFile, err := os.Open(options.Input)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer readFile.Close()

	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)
	for fileScanner.Scan() {
		if !callback(fileScanner.Text()) {
			break
		}
	}
}

func ReadInputJSONLines[T interface{}](options *Options, callback func(T) bool) {
	jsonFile, err := os.Open(options.Input)
	if err != nil {
		log.Fatalf("%v", err)
	}
	dec := json.NewDecoder(jsonFile)
	for {
		var input T
		err := dec.Decode(&input)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("%v", err)
		}
		if !callback(input) {
			break
		}
	}
}

// ParseOptions parses the command line flags provided by a user
func ParseOptions(options *Options) {
	flag.Parse()

	if flag.CommandLine.NArg() > 0 {
		args := flag.CommandLine.Args()
		options.ExtraFlags = args[:len(args)-1]
		options.Input = args[len(args)-1]
	}

	if options.ExtraHelp {
		cmd := exec.Command(options.BinPath, "-h")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			log.Fatalf("Failed to run scanner: %v", err)
		}
		exe := os.Args[0]
		fmt.Println(`
## Note ##
In order to pass any of these flags to the scanner, append them to the end of the command line, after "--".

Normal: ` + exe + ` ... /path/to/input.txt
Extra flags: ` + exe + ` ... -- -extra -flags /path/to/input.txt`)
		// same exit code as normal help
		os.Exit(2)
	}
}
