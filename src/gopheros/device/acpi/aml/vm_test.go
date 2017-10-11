package aml

import (
	"os"
	"testing"
)

func TestVMInit(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resolver := &mockResolver{
			tableFiles: []string{"DSDT.aml"},
		}

		vm := NewVM(os.Stderr, resolver)
		if err := vm.Init(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("parse error", func(t *testing.T) {
		resolver := &fixedPayloadResolver{
			// invalid payload (incomplete opcode)
			payload: []byte{extOpPrefix},
		}

		vm := NewVM(os.Stderr, resolver)
		if err := vm.Init(); err != errParsingAML {
			t.Fatalf("expected Init() to return errParsingAML; got %v", err)
		}
	})
}
