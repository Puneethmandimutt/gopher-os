package aml

import "gopheros/kernel"

// opHandler is a function that implements an AML opcode.
type opHandler func(*execContext, Entity) *kernel.Error

// populateJumpTable assigns the functions that implement the various AML
// opcodes to the VM's jump table.
func (vm *VM) populateJumpTable() {
	for i := 0; i < len(vm.jumpTable); i++ {
		vm.jumpTable[i] = opExecNotImplemented
	}
}

// opExecNotImplemented is a placeholder handler that returns a non-implemented
// opcode error.
func opExecNotImplemented(_ *execContext, ent Entity) *kernel.Error {
	return &kernel.Error{
		Module:  "acpi_aml_vm",
		Message: "opcode " + ent.getOpcode().String() + " not implemented",
	}
}
