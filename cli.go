package revealgo

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	flags "github.com/jessevdk/go-flags"
)

type CLI struct {
}

type CLIOptions struct {
	Port         int    `short:"p" long:"port" description:"tcp port number of this server. default is 3000."`
	Theme        string `long:"theme" description:"slide theme or original css file name. default themes: beige, black, blood, league, moon, night, serif, simple, sky, solarized, and white" default:"cloudops.css"`
	Transition   string `long:"transition" description:"transition effect for slides: default, cube, page, concave, zoom, linear, fade, none" default:"linear"`
	Watermark    bool   `short:"w" long:"watermark" description:"watermark print-pdf option" default:"false"`
	DisablePrint bool   `long:"disableprint" description:"remove the ability print-pdf" default:"false"`
	CredsFile    string `long:"creds_file" description:"the google sheets credentials file. default is 'google-service-account.json'" default:"google-service-account.json"`
	Spreadsheet  string `long:"spreadsheet" description:"the spreadsheet ID where the passwords are stored. example is '1-eV-Np3wbvsCLbHkufTZm_npZsSGhr9nB-MLzdp-nJU'" default:""`
	Worksheet    string `long:"worksheet" description:"the 'worksheet' in the spreadsheet where the passwords are stored. example is 'docker-ws'" default:""`
	PassColumn   string `long:"pass_col" description:"the 'password' column in the spreadsheet. default is 'A'" default:"A"`
	ExpireColumn string `long:"expire_col" description:"the 'expires' column in the spreadsheet. default is 'B'" default:"B"`
	VersionFlag  bool   `short:"v" long:"version" description:"the current version of the revealgo binary" default:"false"`
	HelpFlag     bool   `short:"h" long:"help" description:"show this help screen" default:"false"`
}

func (cli *CLI) Run() {
	opts, args, err := parseOptions()

	if err != nil {
		fmt.Printf("error:%v\n", err)
		os.Exit(1)
	}

	if opts.HelpFlag {
		showHelp()
		os.Exit(0)
	}

	if opts.VersionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}

	if len(args) < 1 {
		showHelp()
		os.Exit(0)
	}

	_, err = os.Stat(opts.Theme)
	originalTheme := false
	if err == nil {
		originalTheme = true
	}

	server := Server{
		port: opts.Port,
	}
	paths := make([]Path, 0)
	for i := range args {
		paths = append(paths, Path{args[i], filepath.Dir(args[i])})
	}
	param := ServerParam{
		Paths:         paths,
		Theme:         addExtention(opts.Theme, "css"),
		Transition:    opts.Transition,
		Watermark:     opts.Watermark,
		DisablePrint:  opts.DisablePrint,
		OriginalTheme: originalTheme,
		CredsFile:     opts.CredsFile,
		Spreadsheet:   opts.Spreadsheet,
		Worksheet:     opts.Worksheet,
		PassColumn:    opts.PassColumn,
		ExpireColumn:  opts.ExpireColumn,
	}
	server.Serve(param)
}

func showHelp() {
	fmt.Fprint(os.Stderr, `Usage: revealgo [options] [markdown_file ...]

Options:
`)
	t := reflect.TypeOf(CLIOptions{})
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag
		var o string
		if s := tag.Get("short"); s != "" {
			o = fmt.Sprintf("-%s, --%s", tag.Get("short"), tag.Get("long"))
		} else {
			o = fmt.Sprintf("--%s", tag.Get("long"))
		}
		fmt.Fprintf(os.Stderr, "  %-21s %s\n", o, tag.Get("description"))
	}
}

func parseOptions() (*CLIOptions, []string, error) {
	opts := &CLIOptions{}
	p := flags.NewParser(opts, flags.PrintErrors)
	args, err := p.Parse()
	if err != nil {
		return nil, nil, err
	}
	return opts, args, nil
}

func addExtention(path string, ext string) string {
	if strings.HasSuffix(path, fmt.Sprintf(".%s", ext)) {
		return path
	}
	path = fmt.Sprintf("%s.%s", path, ext)
	return path
}
