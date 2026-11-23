// Package core holds shared terminal helpers and the application banner.
package core

import "fmt"

// Color holds ANSI escape codes, mirroring the original TerminalColor class.
var Color = struct {
	Black, Red, Green, Orange, Blue, Purple, Reset, Cyan,
	LightGrey, DarkGrey, LightRed, LightGreen, Yellow,
	LightBlue, Pink, LightCyan string
}{
	Black:      "\033[30m",
	Red:        "\033[31m",
	Green:      "\033[32m",
	Orange:     "\033[33m",
	Blue:       "\033[34m",
	Purple:     "\033[35m",
	Reset:      "\033[0m",
	Cyan:       "\033[36m",
	LightGrey:  "\033[37m",
	DarkGrey:   "\033[90m",
	LightRed:   "\033[91m",
	LightGreen: "\033[92m",
	Yellow:     "\033[93m",
	LightBlue:  "\033[94m",
	Pink:       "\033[95m",
	LightCyan:  "\033[96m",
}

// Banner prints the GReal ASCII banner.
func Banner() {
	fmt.Printf(`
 .d8888b.  8888888b.                    888
d88P  Y88b 888   Y88b                   888
888    888 888    888                   888
888        888   d88P  .d88b.   8888b.  888
888  88888 8888888P'  d8P  Y8b     '88b 888
888    888 888 T88b   88888888 .d888888 888
Y88b  d88P 888  T88b  Y8b.     888  888 888
 'Y8888P88 888   T88b  'Y8888  'Y888888 888

Coded by:%[1]s sp34rh34d %[2]s
twitter: %[1]s@spearh34d%[2]s
Welcome to GReal v1.3.0
======================================================================================================
`, Color.Red, Color.Reset)
}
