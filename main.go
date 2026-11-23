// greal is a Go reimplementation of GReal, an email-account enumeration tool
// for authorized security assessments.
//
// Usage:
//
//	greal <module> --<technique> -d <domain> [args]
//
// Modules: gsuite, o365, generic. Techniques: --rcpt, --vrfy, --expn (SMTP);
// --web (o365 only). --verbose prints every probe.
package main

import (
	"fmt"
	"os"
	"strconv"

	"greal/internal/core"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "1.3.0"

func main() {
	for _, a := range os.Args[1:] {
		switch a {
		case "--version", "-V":
			fmt.Printf("greal %s\n", version)
			return
		case "--help", "-h":
			core.Help()
			return
		}
	}

	args, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s%v%s\n", core.Color.Red, err, core.Color.Reset)
		os.Exit(2)
	}

	provider, ok := resolveProvider(args.Module)
	if !ok {
		if args.Module == "" {
			fmt.Fprintf(os.Stderr, "%sA module is required: gsuite, o365 or generic%s\n", core.Color.Red, core.Color.Reset)
		} else {
			fmt.Fprintf(os.Stderr, "%sUnknown module: %s (use gsuite, o365 or generic)%s\n", core.Color.Red, args.Module, core.Color.Reset)
		}
		os.Exit(2)
	}

	technique, ok := resolveTechnique(args.Technique)
	if !ok {
		fmt.Fprintf(os.Stderr, "%sA technique flag is required: --rcpt, --vrfy, --expn or --web%s\n", core.Color.Red, core.Color.Reset)
		os.Exit(2)
	}

	core.Run(provider, technique, args)
}

func resolveProvider(m string) (core.Provider, bool) {
	switch m {
	case "gsuite", "gmail", "gsuit":
		return core.ProviderGSuite, true
	case "o365", "office365", "microsoft":
		return core.ProviderO365, true
	case "generic", "smtp":
		return core.ProviderGeneric, true
	}
	return "", false
}

func resolveTechnique(t string) (core.Technique, bool) {
	switch t {
	case "rcpt":
		return core.TechRCPT, true
	case "vrfy":
		return core.TechVRFY, true
	case "expn", "extn":
		return core.TechEXPN, true
	case "web":
		return core.TechWeb, true
	}
	return "", false
}

// parseArgs parses the positional module plus the flag set.
func parseArgs(argv []string) (core.Args, error) {
	a := core.Args{
		FirstnameList: "wordlist/firstname.txt",
		LastnameList:  "wordlist/lastname.txt",
		Format:        "1",
		Threads:       10,
	}

	// needValue returns the inline "=value" or the next token as the flag value.
	needValue := func(i *int, name, inline string, hasInline bool) (string, error) {
		if hasInline {
			return inline, nil
		}
		if *i+1 >= len(argv) {
			return "", fmt.Errorf("missing value for %s", name)
		}
		*i++
		return argv[*i], nil
	}

	setTechnique := func(t string) error {
		if a.Technique != "" && a.Technique != t {
			return fmt.Errorf("only one technique may be selected (%s vs %s)", a.Technique, t)
		}
		a.Technique = t
		return nil
	}

	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		name, inline, hasInline := splitFlag(tok)

		switch name {
		case "--rcpt", "--vrfy", "--expn", "--extn", "--web":
			if err := setTechnique(name[2:]); err != nil {
				return a, err
			}
		case "-v", "--verbose":
			a.Verbose = true
		case "-d", "--domain":
			v, err := needValue(&i, name, inline, hasInline)
			if err != nil {
				return a, err
			}
			a.Domain = v
		case "-f", "--firtsname-list", "--firstname-list":
			v, err := needValue(&i, name, inline, hasInline)
			if err != nil {
				return a, err
			}
			a.FirstnameList = v
		case "-l", "--lastname-list":
			v, err := needValue(&i, name, inline, hasInline)
			if err != nil {
				return a, err
			}
			a.LastnameList = v
		case "--format":
			v, err := needValue(&i, name, inline, hasInline)
			if err != nil {
				return a, err
			}
			a.Format = v
		case "-t", "--threads":
			v, err := needValue(&i, name, inline, hasInline)
			if err != nil {
				return a, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return a, fmt.Errorf("invalid threads value: %s", v)
			}
			a.Threads = n
		case "-k", "--mail-rate":
			a.MailRate = true
		case "-o", "--output":
			v, err := needValue(&i, name, inline, hasInline)
			if err != nil {
				return a, err
			}
			a.OutputFile = v
		case "--max-per-conn":
			v, err := needValue(&i, name, inline, hasInline)
			if err != nil {
				return a, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return a, fmt.Errorf("invalid max-per-conn value: %s", v)
			}
			a.MaxPerConn = n
		default:
			if len(tok) > 0 && tok[0] == '-' {
				return a, fmt.Errorf("unknown argument: %s", tok)
			}
			// Positional: the module.
			if a.Module == "" {
				a.Module = tok
			} else {
				return a, fmt.Errorf("unexpected argument: %s", tok)
			}
		}
	}

	return a, nil
}

// splitFlag splits "--name=value" into its parts. For non-flag tokens it returns
// the token as the name with no inline value.
func splitFlag(tok string) (name, inline string, hasInline bool) {
	for i := 0; i < len(tok); i++ {
		if tok[i] == '=' {
			return tok[:i], tok[i+1:], true
		}
	}
	return tok, "", false
}
