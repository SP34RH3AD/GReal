package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

// Provider is the target mail platform (the "module").
type Provider string

const (
	ProviderGSuite  Provider = "gsuite"
	ProviderO365    Provider = "o365"
	ProviderGeneric Provider = "generic"
)

// Technique is how a candidate is probed.
type Technique string

const (
	TechRCPT Technique = "rcpt"
	TechVRFY Technique = "vrfy"
	TechEXPN Technique = "expn"
	TechWeb  Technique = "web"
)

var providerNames = map[Provider]string{
	ProviderGSuite:  "Google Workspace (GSuite)",
	ProviderO365:    "Microsoft 365 (O365)",
	ProviderGeneric: "Generic SMTP",
}

var techniqueNames = map[Technique]string{
	TechRCPT: "SMTP RCPT TO",
	TechVRFY: "SMTP VRFY",
	TechEXPN: "SMTP EXPN",
	TechWeb:  "Web (GetCredentialType)",
}

// Args holds the parsed command-line options.
type Args struct {
	Domain        string
	FirstnameList string
	LastnameList  string
	Format        string
	Threads       int
	Module        string
	Technique     string
	Help          bool
	MailRate      bool
	OutputFile    string
	MaxPerConn    int
	Verbose       bool
}

// options is the resolved per-run configuration.
type options struct {
	Provider      Provider
	Technique     Technique
	TargetDomain  string
	FirstnameList string
	LastnameList  string
	TargetMX      string
	Threads       int
	Format        string
	MailRate      bool
	Help          bool
	OutputFile    string
	MaxPerConn    int
	Verbose       bool
}

// Run is the entry point: it runs the chosen technique against the chosen
// provider (module).
func Run(provider Provider, technique Technique, args Args) {
	o := options{
		Provider:      provider,
		Technique:     technique,
		TargetDomain:  args.Domain,
		FirstnameList: args.FirstnameList,
		LastnameList:  args.LastnameList,
		Format:        args.Format,
		Threads:       args.Threads,
		MailRate:      args.MailRate,
		Help:          args.Help,
		OutputFile:    args.OutputFile,
		MaxPerConn:    args.MaxPerConn,
		Verbose:       args.Verbose,
	}

	if o.Help {
		helpMenu()
	}

	if o.TargetDomain == "" {
		fmt.Printf("%sTarget domain is requered!%s\n", Color.Red, Color.Reset)
		os.Exit(1)
	}
	if o.FirstnameList == "" {
		o.FirstnameList = "wordlist/firstname.txt"
	}
	if o.LastnameList == "" {
		o.LastnameList = "wordlist/lastname.txt"
	}
	if !validDomain(o.TargetDomain) {
		fmt.Printf("%sInvalid domain!%s\n", Color.Red, Color.Reset)
		os.Exit(1)
	}

	// Validate the module/technique combination.
	if o.Technique == TechWeb && o.Provider != ProviderO365 {
		fmt.Printf("%sWeb enumeration is only available for the o365 module.%s\n", Color.Red, Color.Reset)
		fmt.Printf("%sNo reliable unauthenticated web enumeration exists for %s; use --rcpt instead.%s\n",
			Color.Orange, providerNames[o.Provider], Color.Reset)
		os.Exit(1)
	}

	Banner()
	o.banner()

	// Web technique (O365) needs no MX or SMTP connection.
	if o.Technique == TechWeb {
		if !isO365Domain(o.TargetDomain) {
			fmt.Printf("[%s!%s] %sWarning: %s does not look like a Microsoft 365 domain; results may be empty.%s\n",
				Color.Blue, Color.Reset, Color.Orange, o.TargetDomain, Color.Reset)
		}
		o.enumerate(o.webWorker)
		return
	}

	// SMTP techniques: resolve the MX for the chosen module.
	o.TargetMX = o.resolveMX()

	// Validate connectivity to the chosen MX server.
	if _, err := smtpResult(o.TargetMX, o.TargetDomain, "test@"+o.TargetDomain); err != nil {
		fmt.Printf("%sInvalid connection to MX server %s!%s\n", Color.Red, o.TargetMX, Color.Reset)
		os.Exit(1)
	}

	// The non-existing-email sanity check applies to the RCPT technique.
	if o.Technique == TechRCPT && !o.MailRate {
		if o.nonExistingEmail() {
			os.Exit(0)
		}
	}

	o.enumerate(o.worker)
}

// resolveMX picks the MX server for the module: auto for gsuite/o365, prompt for
// the generic module.
func (o options) resolveMX() string {
	switch o.Provider {
	case ProviderGSuite:
		if !isGoogleDomain(o.TargetDomain) {
			fmt.Printf("[%s!%s] %sWarning: %s does not use Google MX; enumeration may be unreliable.%s\n",
				Color.Blue, Color.Reset, Color.Orange, o.TargetDomain, Color.Reset)
		}
		mx, err := googleMX(o.TargetDomain)
		if err != nil {
			fmt.Printf("%sCould not resolve a Google MX for %s: %v%s\n", Color.Red, o.TargetDomain, err, Color.Reset)
			os.Exit(1)
		}
		fmt.Printf("%sAuto-selected Google MX: %s%s%s\n", Color.Orange, Color.Green, mx, Color.Reset)
		return mx

	case ProviderO365:
		if !isO365Domain(o.TargetDomain) {
			fmt.Printf("[%s!%s] %sWarning: %s does not look like a Microsoft 365 domain; enumeration may be unreliable.%s\n",
				Color.Blue, Color.Reset, Color.Orange, o.TargetDomain, Color.Reset)
		}
		mx, err := o365MX(o.TargetDomain)
		if err != nil {
			fmt.Printf("%sCould not resolve an O365 MX for %s: %v%s\n", Color.Red, o.TargetDomain, err, Color.Reset)
			os.Exit(1)
		}
		fmt.Printf("%sAuto-selected O365 MX: %s%s%s\n", Color.Orange, Color.Green, mx, Color.Reset)
		return mx

	default: // generic: prompt
		fmt.Printf("%sGetting MX servers for %s%s \n", Color.Orange, o.TargetDomain, Color.Reset)
		mxList, _ := lookupMX(o.TargetDomain)
		for _, mx := range mxList {
			fmt.Printf("[%s+%s] %s\n", Color.Green, Color.Reset, mx)
		}
		fmt.Printf("%sSelect MX server:%s ", Color.Orange, Color.Reset)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		mx := strings.TrimSpace(line)
		if mx == "" {
			fmt.Printf("%sInvalid MX server!%s\n", Color.Red, Color.Reset)
			os.Exit(1)
		}
		return mx
	}
}

// banner prints the per-run configuration summary.
func (o options) banner() {
	msg := fmt.Sprintf(`- Target domain: %[1]s%[2]s%[3]s
- Module: %[1]s%[4]s%[3]s
- Technique: %[1]s%[5]s%[3]s
- Format: %[1]s%[6]s%[3]s
- Threads: %[1]s%[7]d%[3]s`,
		Color.Green, o.TargetDomain, Color.Reset,
		providerNames[o.Provider], techniqueNames[o.Technique], o.Format, o.Threads)

	switch o.Format {
	case "1", "2":
		msg += fmt.Sprintf("\n- Firstname file: %[1]s%[2]s%[3]s\n- Lastname file: %[1]s%[4]s%[3]s",
			Color.Green, o.FirstnameList, Color.Reset, o.LastnameList)
	case "3", "4":
		msg += fmt.Sprintf("\n- Lastname file: %s%s%s", Color.Green, o.LastnameList, Color.Reset)
	case "5":
		msg += fmt.Sprintf("\n- Firstname file: %s%s%s", Color.Green, o.FirstnameList, Color.Reset)
	}

	if o.OutputFile != "" {
		msg += fmt.Sprintf("\n- Output file: %s%s%s", Color.Green, o.OutputFile, Color.Reset)
	}
	if o.MaxPerConn > 0 {
		msg += fmt.Sprintf("\n- Max probes/connection: %s%d%s", Color.Green, o.MaxPerConn, Color.Reset)
	}

	fmt.Printf("%s\n======================================================================================================\n", msg)
}

// nonExistingEmail probes a random address to detect MX servers that answer OK
// for every recipient. Returns true if the server is unreliable for this mode.
func (o options) nonExistingEmail() bool {
	fmt.Printf("[%s!%s] %sTesting MX server for non existing email response...%s\n",
		Color.Blue, Color.Reset, Color.Orange, Color.Reset)

	result, err := smtpResult(o.TargetMX, o.TargetDomain, "wakk4k23lem23@"+o.TargetDomain)
	if err != nil {
		return false
	}
	if strings.Contains(result, "OK") {
		fmt.Printf("%sThe MX server [%s] response with status OK for non existing email.%s\n",
			Color.Red, o.TargetMX, Color.Reset)
		fmt.Printf("%sTry adding -k arg, example: ./greal %s --rcpt -d %s --format %s -k %s\n",
			Color.Orange, o.Provider, o.TargetDomain, o.Format, Color.Reset)
		return true
	}
	return false
}

// candidates builds the candidate list for the selected format.
func (o options) candidates() []string {
	switch o.Format {
	case "1":
		return o.format1()
	case "2":
		return o.format2()
	case "3":
		return o.format3()
	case "4":
		return o.format4()
	case "5":
		return o.format5()
	}
	return nil
}

// enumerate runs the given worker function across a bounded pool, then prints a
// summary and writes the output file.
func (o options) enumerate(work func(jobs <-chan string, c *collector)) {
	accounts := o.candidates()

	workers := o.Threads
	if workers < 1 {
		workers = 1
	}

	c := &collector{}
	jobs := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			work(jobs, c)
		}()
	}
	for _, account := range accounts {
		jobs <- account
	}
	close(jobs)
	wg.Wait()

	found := c.list()
	o.summary(found, len(accounts))

	if err := o.writeOutput(found, len(accounts)); err != nil {
		fmt.Printf("%sFailed to write output file: %v%s\n", Color.Red, err, Color.Reset)
	} else if o.OutputFile != "" {
		fmt.Printf("%s[CONFIDENCIAL] Results written to %s%s\n", Color.Orange, o.OutputFile, Color.Reset)
	}
}

// worker drains jobs on a single SMTP session, recycling the connection after
// MaxPerConn probes (when set) to avoid per-connection rate limits.
func (o options) worker(jobs <-chan string, c *collector) {
	sess, err := NewSession(o.TargetMX, o.TargetDomain)
	if err != nil {
		sess = &Session{mx: o.TargetMX, domain: o.TargetDomain} // Probe reconnects lazily
	}
	defer sess.Close()

	count := 0
	for account := range jobs {
		if o.MaxPerConn > 0 && count >= o.MaxPerConn {
			sess.Close()
			count = 0
		}
		o.run(sess, account, c)
		count++
	}
}

// webWorker drains jobs issuing HTTP-based probes (no SMTP session).
func (o options) webWorker(jobs <-chan string, c *collector) {
	for account := range jobs {
		target := account + "@" + o.TargetDomain
		fmt.Printf("processing: %s                                                   \r", target)

		var hit bool
		var err error
		switch o.Provider {
		case ProviderO365:
			hit, err = o365AccountExists(target)
		}
		if err != nil {
			if o.Verbose {
				fmt.Printf("%s[v] %-45s error: %v%s\n", Color.DarkGrey, target, err, Color.Reset)
			}
			continue
		}
		if o.Verbose {
			status := "not found"
			if hit {
				status = "exists"
			}
			fmt.Printf("%s[v] %-45s %s%s\n", Color.DarkGrey, target, status, Color.Reset)
		}
		if hit {
			c.add(target)
			fmt.Printf("[%s+%s] %s%s%s                                                                            \n",
				Color.Green, Color.Reset, Color.Green, target, Color.Reset)
		}
	}
}

// run probes a single candidate using the technique's SMTP command, then prints
// and records it when the response suggests a hit.
func (o options) run(sess *Session, account string, c *collector) {
	var target, result string
	var err error

	switch o.Technique {
	case TechVRFY:
		target = account + "@" + o.TargetDomain
		fmt.Printf("processing: VRFY %s                                                   \r", target)
		result, err = sess.VRFY(target)
	case TechEXPN:
		target = account // EXPN takes a list/alias name, not an address
		fmt.Printf("processing: EXPN %s                                                   \r", target)
		result, err = sess.EXPN(target)
	default: // TechRCPT
		target = account + "@" + o.TargetDomain
		fmt.Printf("processing: %s                                                   \r", target)
		result, err = sess.Probe(target)
	}
	if err != nil {
		if o.Verbose {
			fmt.Printf("%s[v] %-45s error: %v%s\n", Color.DarkGrey, target, err, Color.Reset)
		}
		return
	}
	if o.Verbose {
		fmt.Printf("%s[v] %-45s %s%s\n", Color.DarkGrey, target, strings.TrimSpace(result), Color.Reset)
	}

	if o.found(target, result, c) {
		fmt.Printf("[%s+%s] %s%s%s                                                                            \n",
			Color.Green, Color.Reset, Color.Green, target, Color.Reset)
	}
}

// found applies the technique-specific success heuristic. On a hit it records the
// target and returns true.
func (o options) found(target, result string, c *collector) bool {
	switch o.Technique {
	case TechVRFY:
		if code := respCode(result); code == 250 || code == 251 {
			c.add(target)
			return true
		}
		return false

	case TechEXPN:
		if respCode(result) == 250 {
			c.add(target)
			return true
		}
		return false

	default: // TechRCPT
		if o.MailRate {
			if strings.Contains(result, "receiving mail at a rate") {
				c.add(target)
				return true
			}
			return false
		}
		if !strings.Contains(result, "not exist.") && !strings.Contains(result, "Access denied.") {
			if strings.Contains(result, "Server busy.") {
				fmt.Printf("%sThe MX server [%s] response with status 500 Server busy, please try later.%s\n",
					Color.Red, o.TargetMX, Color.Reset)
				os.Exit(1)
			}
			c.add(target)
			return true
		}
		return false
	}
}

// summary prints a final run summary.
func (o options) summary(found []string, totalProbed int) {
	fmt.Printf("%80s\r", "")
	fmt.Printf("======================================================================================================\n")
	fmt.Printf("%sDone.%s Module: %s%s%s  Technique: %s%s%s  Probed: %s%d%s  Found: %s%d%s\n",
		Color.Orange, Color.Reset,
		Color.Green, o.Provider, Color.Reset,
		Color.Green, o.Technique, Color.Reset,
		Color.Green, totalProbed, Color.Reset,
		Color.Green, len(found), Color.Reset)
}

// --- candidate list generators ---

func (o options) format1() []string {
	firsts := readLines(o.FirstnameList)
	lasts := readLines(o.LastnameList)
	var out []string
	for _, f := range firsts {
		for _, l := range lasts {
			out = append(out, fmt.Sprintf("%s.%s", f, l))
		}
	}
	return out
}

func (o options) format2() []string {
	firsts := readLines(o.FirstnameList)
	lasts := readLines(o.LastnameList)
	var out []string
	for _, f := range firsts {
		for _, l := range lasts {
			out = append(out, f+l)
		}
	}
	return out
}

func (o options) format3() []string {
	lasts := readLines(o.LastnameList)
	var out []string
	for c := 'a'; c <= 'z'; c++ {
		for _, l := range lasts {
			out = append(out, fmt.Sprintf("%c.%s", c, l))
		}
	}
	return out
}

func (o options) format4() []string {
	lasts := readLines(o.LastnameList)
	var out []string
	for c := 'a'; c <= 'z'; c++ {
		for _, l := range lasts {
			out = append(out, fmt.Sprintf("%c%s", c, l))
		}
	}
	return out
}

func (o options) format5() []string {
	return readLines(o.FirstnameList)
}

// readLines reads non-empty, trimmed lines from a file, exiting on error.
func readLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("%sCannot read file %s: %v%s\n", Color.Red, path, err, Color.Reset)
		os.Exit(1)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func helpMenu() {
	Help()
	os.Exit(0)
}

// Help prints the help menu.
func Help() {
	fmt.Print(`GReal enumeration - Help menu

Usage:
  ./greal <module> --<technique> -d <domain> [args]

Modules (platform to target):
	gsuite      Google Workspace  (SMTP against the auto-detected Google MX)
	o365        Microsoft 365     (web, or SMTP against the auto-detected O365 MX)
	generic     any mail server   (SMTP, prompts for the MX server)

Techniques (choose one):
	--rcpt      SMTP RCPT TO recipient enumeration (generic, most reliable)
	--vrfy      SMTP VRFY user verification (often disabled on modern servers)
	--expn      SMTP EXPN mailing-list / alias expansion
	--web       Web enumeration via GetCredentialType (o365 module only)

Args
	-d, --domain              set target domain (required)
	-f, --firstname-list      set firstname list file
	-l, --lastname-list       set lastname list file
	-t, --threads             set threads
	    --format              set what kind of email format use for enumeration
		                      [ 1 - Firstname.Lastname ]
		                      [ 2 - FirstnameLastname ]
		                      [ 3 - F.Lastname ]
		                      [ 4 - FLastname ]
		                      [ 5 - firstname ]

	-k, --mail-rate           (rcpt) Ignore 'OK' response and show only email with 'receiving mail at a rate ' status response
	-o, --output              write found accounts to a file (.json / .csv / .txt by extension, CONFIDENCIAL)
	    --max-per-conn        recycle each SMTP connection after N probes (0 = keep connection open)
	-v, --verbose             print every probe result (status/code), not just hits
	-V, --version             show version
	-h, --help                show this message

Examples:

	Google Workspace, RCPT, format [ Firstname.Lastname ]
	use: ./greal gsuite --rcpt -d domain.com --format 1 -f wordlist/firstname.txt -l wordlist/lastname.txt

	Microsoft 365, web enumeration, with output file
	use: ./greal o365 --web -d domain.com --format 1 -o results.json

	Microsoft 365, SMTP RCPT, verbose
	use: ./greal o365 --rcpt -d domain.com --format 4 -l wordlist/lastname.txt --verbose

NOTE: offensive testing against client assets requires written authorization and
a defined scope. Web modes send candidate addresses to Microsoft/Google.
`)
}
