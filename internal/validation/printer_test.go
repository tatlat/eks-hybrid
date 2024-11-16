package validation_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/aws/eks-hybrid/internal/validation"
)

func TestPrinter(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	var buf bytes.Buffer
	printer := validation.NewPrinter(
		validation.WithNoColor(),
		validation.WithOutWriter(&buf),
	)

	printer.Starting(ctx, "test", "Starting test")
	printer.Done(ctx, "test", nil)
	printer.Starting(ctx, "test", "Validating next")
	printer.Done(ctx, "test", errors.New("error message"))
	printer.Starting(ctx, "test", "Third validation")
	printer.Done(ctx, "test", validation.NewRemediableErr("another error message", "you can fix this"))
	printer.Starting(ctx, "test", "Fourth validation")
	printer.Done(ctx, "test",
		errors.Join(
			errors.New("error message"),
			validation.NewRemediableErr("another error message", "you can fix this"),
			errors.New("error message"),
		),
	)

	output := buf.String()
	want := `* Starting test [Success]
* Validating next [Failed]
  └─ Error
     └─ error message
* Third validation [Failed]
  └─ Error
     └─ another error message
     └─ Remediation
        └─ you can fix this
* Fourth validation [Failed]
  ├─ Error
  │  └─ error message
  ├─ Error
  │  └─ another error message
  │  └─ Remediation
  │     └─ you can fix this
  └─ Error
     └─ error message
`

	t.Log("Want\n" + want)
	t.Log("Got\n" + output)
	g.Expect(output).To(BeComparableTo(want))
}

func TestPrinterWithExternalLogs(t *testing.T) {
	testCases := []struct {
		name     string
		starting string
		logs     []string
		err      error
		want     string
	}{
		{
			name:     "happy path",
			starting: "Starting first validation",
			want:     "* Starting first validation [Success]\n",
		},
		{
			name:     "with one error, no remediation and logs",
			starting: "Second validation",
			logs:     []string{"First log", "Second log"},
			err:      errors.New("second validation failed"),
			want: `* Second validation [Failed]
  └─ Error
     ├─ second validation failed
     └─ stderr
        ├─ First log
        └─ Second log
`,
		},
		{
			name:     "with one error, with remediation and logs",
			starting: "Third validation",
			logs:     []string{"First log", "Second log"},
			err:      validation.NewRemediableErr("third validation error", "you can fix this"),
			want: `* Third validation [Failed]
  └─ Error
     ├─ third validation error
     ├─ stderr
     │  ├─ First log
     │  └─ Second log
     └─ Remediation
        └─ you can fix this
`,
		},
		{
			name:     "with multiple errors, with remediation and logs",
			starting: "Fourth validation",
			logs:     []string{"First log", "Second log", "Third log"},
			err: errors.Join(
				errors.New("error message"),
				validation.NewRemediableErr("another error message", "you can fix this"),
				errors.New("error message"),
			),
			want: `* Fourth validation [Failed]
  ├─ Error
  │  └─ error message
  ├─ Error
  │  └─ another error message
  │  └─ Remediation
  │     └─ you can fix this
  └─ Error
     ├─ error message
     └─ stderr
        ├─ First log
        ├─ Second log
        └─ Third log
`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()
			logs := make(chan string, 10)
			external := validation.NewChannelReader(logs, "stderr")
			var buf bytes.Buffer
			printer := validation.NewPrinter(
				validation.WithNoColor(),
				validation.WithOutWriter(&buf),
				validation.WithExternalLogs(external),
			)

			printer.Starting(ctx, tc.name, tc.starting)
			for _, log := range tc.logs {
				logs <- log
			}
			printer.Done(ctx, tc.name, tc.err)

			output := buf.String()
			t.Log("Want\n" + tc.want)
			t.Log("Got\n" + output)
			g.Expect(output).To(BeComparableTo(tc.want))
		})
	}
}
