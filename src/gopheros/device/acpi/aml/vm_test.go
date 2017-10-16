package aml

import (
	"gopheros/kernel"
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

func TestVMObjectLookups(t *testing.T) {
	resolver := &mockResolver{
		tableFiles: []string{"DSDT.aml"},
	}
	vm := NewVM(os.Stderr, resolver)
	if err := vm.Init(); err != nil {
		t.Fatal(err)
	}

	specs := []struct {
		absPath string
		match   bool
	}{
		{
			``,
			false,
		},
		{
			`\`,
			true,
		},
		{
			`\_SB_.PCI0.SBRG.PIC_`,
			true,
		},
		{
			`\_SB_.PCI0.UNKNOWN_PATH`,
			false,
		},
	}

	for specIndex, spec := range specs {
		foundMatch := vm.Lookup(spec.absPath) != nil
		if foundMatch != spec.match {
			t.Errorf("[spec %d] expected lookup match status to be %t", specIndex, spec.match)
		}
	}
}

func TestVMVisit(t *testing.T) {
	resolver := &mockResolver{
		tableFiles: []string{"parser-testsuite-DSDT.aml"},
	}
	vm := NewVM(os.Stderr, resolver)
	if err := vm.Init(); err != nil {
		t.Fatal(err)
	}

	var (
		methodCount int
		expCount    = 2
	)

	vm.Visit(EntityTypeMethod, func(_ int, ent Entity) bool {
		methodCount++
		return true
	})

	if methodCount != expCount {
		t.Fatalf("expected visitor to be invoked for %d methods; got %d", expCount, methodCount)
	}
}

func TestVMExecBlockControlFlows(t *testing.T) {
	resolver := &mockResolver{}
	vm := NewVM(os.Stderr, resolver)
	if err := vm.Init(); err != nil {
		t.Fatal(err)
	}

	t.Run("sequential ctrl flow", func(t *testing.T) {
		block := &scopeEntity{
			children: []Entity{
				&unnamedEntity{op: opcode(0)},
				&unnamedEntity{op: opcode(0)},
				&unnamedEntity{op: opcode(0)},
				&unnamedEntity{op: opcode(0)},
			},
		}

		var instrExecCount int
		vm.jumpTable[0] = func(ctx *execContext, _ Entity) *kernel.Error {
			instrExecCount++
			return nil
		}

		ctx := new(execContext)
		if err := vm.execBlock(ctx, block); err != nil {
			t.Fatal(err)
		}

		if instrExecCount != len(block.Children()) {
			t.Errorf("expected opcode 0 to be executed %d times; got %d", len(block.Children()), instrExecCount)
		}

		if ctx.ctrlFlow != ctrlFlowTypeNextOpcode {
			t.Errorf("expected ctx.ctrlFlow to be %d; got %d", ctrlFlowTypeNextOpcode, ctx.ctrlFlow)
		}
	})

	t.Run("break ctrl flow", func(t *testing.T) {
		block := &scopeEntity{
			children: []Entity{
				&unnamedEntity{op: opcode(0)},
				&unnamedEntity{op: opcode(0)},
				// These instructions will not be executed due to the break
				&unnamedEntity{op: opcode(0)},
				&unnamedEntity{op: opcode(0)},
				&unnamedEntity{op: opcode(0)},
			},
		}

		var instrExecCount int

		vm.jumpTable[0] = func(ctx *execContext, _ Entity) *kernel.Error {
			instrExecCount++

			// Break out of "loop" after the 2nd instruction
			if instrExecCount == 2 {
				ctx.ctrlFlow = ctrlFlowTypeBreak
			}
			return nil
		}

		ctx := new(execContext)
		if err := vm.execBlock(ctx, block); err != nil {
			t.Fatal(err)
		}

		if exp := 2; instrExecCount != exp {
			t.Errorf("expected opcode 0 to be executed %d times; got %d", exp, instrExecCount)
		}

		// ctrlFlow should switch to NextOpcode so we can run
		// instructions defined after the "loop"
		if ctx.ctrlFlow != ctrlFlowTypeNextOpcode {
			t.Errorf("expected ctx.ctrlFlow to be %d; got %d", ctrlFlowTypeNextOpcode, ctx.ctrlFlow)
		}
	})

	t.Run("continue ctrl flow", func(t *testing.T) {
		block := &scopeEntity{
			children: []Entity{
				&unnamedEntity{op: opcode(0)},
			},
		}

		var instrExecCount int

		vm.jumpTable[0] = func(ctx *execContext, _ Entity) *kernel.Error {
			instrExecCount++

			// Break out of "loop" after the 5th instruction run
			if instrExecCount == 5 {
				ctx.ctrlFlow = ctrlFlowTypeBreak
			} else {
				ctx.ctrlFlow = ctrlFlowTypeContinue
			}
			return nil
		}

		ctx := new(execContext)
		if err := vm.execBlock(ctx, block); err != nil {
			t.Fatal(err)
		}

		if exp := 5; instrExecCount != exp {
			t.Errorf("expected opcode 0 to be executed %d times; got %d", exp, instrExecCount)
		}

		// ctrlFlow should switch to NextOpcode so we can run
		// instructions defined after the "loop"
		if ctx.ctrlFlow != ctrlFlowTypeNextOpcode {
			t.Errorf("expected ctx.ctrlFlow to be %d; got %d", ctrlFlowTypeNextOpcode, ctx.ctrlFlow)
		}
	})

	t.Run("return ctrl flow", func(t *testing.T) {
		block := &scopeEntity{
			children: []Entity{
				&unnamedEntity{op: opcode(0)},
				&unnamedEntity{op: opcode(0)},
				&unnamedEntity{op: opcode(0)},
			},
		}

		var instrExecCount int

		vm.jumpTable[0] = func(ctx *execContext, _ Entity) *kernel.Error {
			instrExecCount++

			// return after 2nd instruction execution
			if instrExecCount == 2 {
				ctx.retVal = "foo"
				ctx.ctrlFlow = ctrlFlowTypeFnReturn
			} else {
				ctx.ctrlFlow = ctrlFlowTypeContinue
			}
			return nil
		}

		ctx := new(execContext)
		if err := vm.execBlock(ctx, block); err != nil {
			t.Fatal(err)
		}

		if exp := 2; instrExecCount != exp {
			t.Errorf("expected opcode 0 to be executed %d times; got %d", exp, instrExecCount)
		}

		if exp := "foo"; ctx.retVal != exp {
			t.Errorf("expected retVal to be %v; got %v", exp, ctx.retVal)
		}

		// ctrlFlow should remain FnReturn so we can exit any nested
		// loops till we reach the invoked function.
		if ctx.ctrlFlow != ctrlFlowTypeFnReturn {
			t.Errorf("expected ctx.ctrlFlow to be %d; got %d", ctrlFlowTypeFnReturn, ctx.ctrlFlow)
		}
	})

	t.Run("instr exec error", func(t *testing.T) {
		block := &scopeEntity{
			children: []Entity{
				&unnamedEntity{op: opcode(0)},
			},
		}

		vm.jumpTable[0] = opExecNotImplemented

		ctx := new(execContext)
		expErr := &kernel.Error{
			Module:  "acpi_aml_vm",
			Message: "opcode Zero not implemented",
		}
		if err := vm.execBlock(ctx, block); err == nil || err.Error() != expErr.Error() {
			t.Errorf("expected to get error: %v; got: %v", expErr, err)
		}
	})
}
