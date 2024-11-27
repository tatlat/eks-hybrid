package validation

import (
	"context"
	"fmt"
	"io"
	"os"
)

// Informer is an informer that prints the validation steps to stdout.
type Printer struct {
	out          io.Writer
	externalLogs LineReader
	color        Colorer
}

var _ Informer = Printer{}

// PrinterOpt allows to configure the Printer.
type PrinterOpt func(*Printer)

func NewPrinter(opts ...PrinterOpt) Printer {
	p := Printer{
		out:   os.Stdout,
		color: Colorer{},
	}

	for _, opt := range opts {
		opt(&p)
	}

	return p
}

// WithNoColor disables color output.
func WithNoColor() PrinterOpt {
	return func(p *Printer) {
		p.color.noColor = true
	}
}

// WithOutWriter allows to configure the output destination.
func WithOutWriter(out io.Writer) PrinterOpt {
	return func(p *Printer) {
		p.out = out
	}
}

// WithExternalLogs allows to configure an external source of logs
// that will get included in the output when a validation fails.
// This is useful for correctly displaying the output of external
// processes that are invoked during the validation. You can just capture
// their output and let the printer handle its display.
func WithExternalLogs(in LineReader) PrinterOpt {
	return func(p *Printer) {
		p.externalLogs = in
	}
}

// Starting prints the starting message of a validation.
func (p Printer) Starting(ctx context.Context, name, message string) {
	p.print("* %s ", message)
}

// Done prints the result of a validation.
// If a validation fails, it will print the error, external output if any and
// the error remediation if the error is Remediable.
func (p Printer) Done(ctx context.Context, name string, err error) {
	if err == nil {
		p.println("[%s]", p.color.Green("Success"))
		return
	}

	p.println("[%s]", p.color.Red("Failed"))

	errs := Unwrap(err)
	for _, e := range errs[:len(errs)-1] {
		p.printError(e)
	}

	p.printLastError(errs[len(errs)-1])
}

func (p Printer) printError(err error) {
	p.println("  ├─ %s", p.color.Red("Error"))
	p.println("  │  └─ %s", err)
	if IsRemediable(err) {
		p.println("  │  └─ %s", p.color.Blue("Remediation"))
		p.println("  │     └─ %s", Remediation(err))
	}
}

func (p Printer) printLastError(err error) {
	external := p.readExternal()

	if len(external) == 0 {
		p.println("  └─ %s", p.color.Red("Error"))
		p.println("     └─ %s", err)
	} else {
		p.println("  └─ %s", p.color.Red("Error"))
		p.println("     ├─ %s", err)
		if IsRemediable(err) {
			p.printExternalLogs("     ├─", "     │", external)
		} else {
			p.printExternalLogs("     └─", "      ", external)
		}
	}

	if IsRemediable(err) {
		p.println("     └─ %s", p.color.Blue("Remediation"))
		p.println("        └─ %s", Remediation(err))
	}
}

func (p Printer) println(msg string, args ...any) {
	fmt.Fprintf(p.out, msg+"\n", args...)
}

func (p Printer) print(msg string, args ...any) {
	fmt.Fprintf(p.out, msg, args...)
}

func (p Printer) readExternal() []string {
	if p.externalLogs == nil {
		return nil
	}

	var lines []string
	for line, ok := p.externalLogs.Line(); ok; line, ok = p.externalLogs.Line() {
		lines = append(lines, line)
	}

	return lines
}

func (p Printer) printExternalLogs(prefixHeader, prefixLog string, lines []string) {
	if len(lines) > 0 {
		p.println("%s %s", prefixHeader, p.color.Yellow(p.externalLogs.Name()))
		for _, line := range lines[:len(lines)-1] {
			p.println("%s  ├─ %s", prefixLog, line)
		}

		p.println("%s  └─ %s", prefixLog, lines[len(lines)-1])
	}
}

type NoOpInformer struct{}

var _ Informer = NoOpInformer{}

func (NoOpInformer) Starting(ctx context.Context, name, message string) {}
func (NoOpInformer) Done(ctx context.Context, name string, err error)   {}

const (
	resetC   = "\033[0m"
	blackC   = "\033[30m"
	redC     = "\033[31m"
	greenC   = "\033[32m"
	yellowC  = "\033[33m"
	blueC    = "\033[34m"
	purpleC  = "\033[35m"
	cyanC    = "\033[36m"
	greyC    = "\033[37m"
	whiteC   = "\033[97m"
	magentaC = "\033[95m"

	underlineC     = "\033[4m"
	resetUnderline = "\033[24m"

	boldC     = "\033[1m"
	resetBold = "\033[22m"
)

type Colorer struct {
	noColor bool
}

func (c Colorer) wrap(m, color, reset string) string {
	if c.noColor {
		return m
	}
	return color + m + reset
}

func (c Colorer) color(m, color string) string {
	return c.wrap(m, color, resetC)
}

func (c Colorer) Blue(m string) string {
	return c.color(m, blueC)
}

func (c Colorer) Cyan(m string) string {
	return c.color(m, cyanC)
}

func (c Colorer) Red(m string) string {
	return c.color(m, redC)
}

func (c Colorer) Green(m string) string {
	return c.color(m, greenC)
}

func (c Colorer) Yellow(m string) string {
	return c.color(m, yellowC)
}

func (c Colorer) Black(m string) string {
	return c.color(m, blackC)
}

func (c Colorer) Grey(m string) string {
	return c.color(m, greyC)
}

func (c Colorer) Magenta(m string) string {
	return c.color(m, magentaC)
}

func (c Colorer) Underline(m string) string {
	return c.wrap(m, underlineC, resetUnderline)
}

func (c Colorer) Bold(m string) string {
	return c.wrap(m, boldC, resetBold)
}
