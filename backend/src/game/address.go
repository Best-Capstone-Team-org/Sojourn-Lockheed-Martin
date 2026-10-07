package game

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RESOLVE SCENARIO REFERENCES
// ResolveAddress resolves AddressRef to address in memory
func validateAddressRef(ref AddressRef) error {
	if ref.Addr != "" {
		if ref.Sym != "" || ref.Field != "" || ref.Offset != 0 {
			return fmt.Errorf("address reference using 'addr' cannot also use sym, field, or offset")
		}
		return nil
	}
	if ref.Sym == "" {
		return fmt.Errorf("address reference requires either 'addr' or 'sym'")
	}
	if ref.Field != "" && ref.Offset != 0 {
		return fmt.Errorf("symbol address reference cannot use both 'field' and 'offset'")
	}
	return nil
}

func (s *Scenario) ResolveAddress(ref AddressRef) (uint64, error) {
	if err := validateAddressRef(ref); err != nil {
		return 0, err
	}
	// If we are given a direct address
	if ref.Addr != "" {
		addr, err := strconv.ParseUint(ref.Addr, 0, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid address %q: %w", ref.Addr, err)
		}
		return addr, nil
	}
	// Otherwise, we need a symbol
	if ref.Sym == "" {
		return 0, fmt.Errorf("address reference has no addr or sym")
	}
	symbolAddr, ok := s.Symbols.Symbols[ref.Sym]
	if !ok {
		return 0, fmt.Errorf("unknown symbol %q", ref.Sym)
	}
	base, err := strconv.ParseUint(symbolAddr, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid address for symbol %q: %w", ref.Sym, err)
	}
	// Symbol + field case
	if ref.Field != "" {
		fields, ok := s.Symbols.Fields[ref.Sym]
		if !ok {
			return 0, fmt.Errorf("no field information for symbol %q", ref.Sym)
		}
		offset, ok := fields[ref.Field]
		if !ok {
			return 0, fmt.Errorf(
				"unknown field %q for symbol %q",
				ref.Field,
				ref.Sym,
			)
		}
		return base + uint64(offset), nil
	}
	// Symbol + offset case
	if ref.Offset != 0 {
		return base + uint64(ref.Offset), nil
	}
	// Just the symbol itself
	return base, nil
}

func (s *Scenario) validateMemoryRange(ref AddressRef, length int) error {
	if length <= 0 {
		return fmt.Errorf("memory range length must be greater than 0")
	}
	addr, err := s.ResolveAddress(ref)
	if err != nil {
		return err
	}
	if addr > math.MaxUint32 {
		return fmt.Errorf("address 0x%X exceeds 32-bit introspection address space", addr)
	}
	if uint64(length-1) > ^uint64(0)-addr {
		return fmt.Errorf(
			"memory range starting at 0x%X with length %d overflows address space",
			addr,
			length,
		)
	}
	end := addr + uint64(length) - 1
	for _, region := range s.MemMap.Regions {
		lo, err := strconv.ParseUint(region.Lo, 0, 64)
		if err != nil {
			return fmt.Errorf(
				"invalid lower address %q for memory region %q",
				region.Lo,
				region.Name,
			)
		}
		hi, err := strconv.ParseUint(region.Hi, 0, 64)
		if err != nil {
			return fmt.Errorf(
				"invalid upper address %q for memory region %q",
				region.Hi,
				region.Name,
			)
		}
		if addr >= lo && end <= hi {
			return nil
		}
	}
	return fmt.Errorf(
		"memory range 0x%X-0x%X is outside the declared memory map",
		addr,
		end,
	)
}

func (s *Scenario) validatePredicateAddresses(predicate Predicate) error {
	var length int
	switch predicate.Op {
	case "mem_u8":
		length = 1
	case "mem_u16":
		length = 2
	case "mem_u32":
		length = 4
	case "mem_bits":
		length = predicate.Width
		if length == 0 {
			length = 4
		}
	case "mem", "mem_changed":
		length = predicate.Len
	}
	if length > 0 && predicate.At != nil {
		if err := s.validateMemoryRange(*predicate.At, length); err != nil {
			return fmt.Errorf(
				"invalid address for %q predicate: %w",
				predicate.Op,
				err,
			)
		}
	}
	if predicate.Op == "script" {
		parts := strings.Split(predicate.Entry, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid script entry %q", predicate.Entry)
		}
		if _, err := os.Stat(filepath.Join(s.Dir, parts[0])); err != nil {
			return fmt.Errorf("script file %q is unavailable: %w", parts[0], err)
		}
	}
	if predicate.Op == "commanded" && predicate.At != nil {
		if _, err := s.ResolveAddress(*predicate.At); err != nil {
			return fmt.Errorf(
				"invalid address for commanded predicate: %w",
				err,
			)
		}
	}
	switch predicate.Op {
	case "all", "any":
		var children []Predicate
		if err := json.Unmarshal(predicate.Of, &children); err != nil {
			return err
		}
		for _, child := range children {
			if err := s.validatePredicateAddresses(child); err != nil {
				return err
			}
		}
	case "not", "ever", "sustained", "within":
		var child Predicate
		if err := json.Unmarshal(predicate.Of, &child); err != nil {
			return err
		}
		if err := s.validatePredicateAddresses(child); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scenario) validateObjectiveAddresses() error {
	for _, objective := range s.Objectives {
		if err := s.validatePredicateAddresses(objective.Success); err != nil {
			return fmt.Errorf(
				"invalid success predicate address for objective %q: %w",
				objective.ID,
				err,
			)
		}
		if objective.Fail != nil {
			if err := s.validatePredicateAddresses(*objective.Fail); err != nil {
				return fmt.Errorf(
					"invalid fail predicate address for objective %q: %w",
					objective.ID,
					err,
				)
			}
		}
		for _, partial := range objective.Partial {
			if err := s.validatePredicateAddresses(partial.When); err != nil {
				return fmt.Errorf(
					"invalid partial predicate address for objective %q: %w",
					objective.ID,
					err,
				)
			}
		}
	}
	return nil
}

func (s *Scenario) validateSetupAddresses() error {
	if s.Setup == nil {
		return nil
	}
	for i, write := range s.Setup.Writes {
		if err := validateAddressRef(write.At); err != nil {
			return fmt.Errorf("setup write %d has invalid address reference: %w", i, err)
		}
		count := 0
		length := 0
		if write.U8 != nil {
			count++
			length = 1
		}
		if write.U16 != nil {
			count++
			length = 2
		}
		if write.U32 != nil {
			count++
			length = 4
		}
		if write.Hex != "" {
			count++
			data, err := hex.DecodeString(write.Hex)
			if err != nil {
				return fmt.Errorf("setup write %d has invalid hex value %q: %w", i, write.Hex, err)
			}
			if len(data) == 0 {
				return fmt.Errorf("setup write %d has empty hex value", i)
			}
			length = len(data)
		}
		if count != 1 {
			return fmt.Errorf("setup write %d must contain exactly one of u8, u16, u32, or hex", i)
		}
		if err := s.validateMemoryRange(write.At, length); err != nil {
			return fmt.Errorf("invalid setup write %d address: %w", i, err)
		}
	}
	return nil
}
