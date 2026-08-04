// Package registry is the single registration point for domain generators.
package registry

import (
	"fmt"
	"sort"
	"strings"

	"yourddo-data-tools/internal/domain"
	"yourddo-data-tools/internal/domain/alchemical"
	"yourddo-data-tools/internal/domain/almostthere"
	"yourddo-data-tools/internal/domain/attunedbyheroism"
	"yourddo-data-tools/internal/domain/catalystcrafting"
	"yourddo-data-tools/internal/domain/dinosaurbone"
	"yourddo-data-tools/internal/domain/finishingtouch"
	"yourddo-data-tools/internal/domain/fountain"
	"yourddo-data-tools/internal/domain/gearplanner"
	"yourddo-data-tools/internal/domain/incrediblepotential"
	"yourddo-data-tools/internal/domain/itemsets"
	"yourddo-data-tools/internal/domain/lostpurpose"
	"yourddo-data-tools/internal/domain/nearlycomplete"
	"yourddo-data-tools/internal/domain/nearlyfinished"
	"yourddo-data-tools/internal/domain/stormreaver"
	"yourddo-data-tools/internal/domain/suppressedpower"
	"yourddo-data-tools/internal/domain/traceofmadness"
	"yourddo-data-tools/internal/domain/viktranium"
	"yourddo-data-tools/internal/domain/zhentarim"
)

type Registration struct {
	Generator domain.Generator
	Aliases   []string
}

// registrations is the only list changed when a domain is added.
var registrations = []Registration{
	{Generator: itemsets.New(), Aliases: []string{"itemsets"}},
	{Generator: gearplanner.New(), Aliases: []string{"gearplanner"}},
	{Generator: zhentarim.New(), Aliases: []string{"zhentarim"}},
	{Generator: nearlycomplete.New()},
	{Generator: fountain.New(), Aliases: []string{"fountain"}},
	{Generator: stormreaver.New()},
	{Generator: nearlyfinished.New()},
	{Generator: almostthere.New()},
	{Generator: finishingtouch.New()},
	{Generator: alchemical.New()},
	{Generator: incrediblepotential.New()},
	{Generator: catalystcrafting.New()},
	{Generator: traceofmadness.New()},
	{Generator: suppressedpower.New()},
	{Generator: lostpurpose.New()},
	{Generator: attunedbyheroism.New(), Aliases: []string{"attuned to heroism"}},
	{Generator: dinosaurbone.New()},
	{Generator: viktranium.New()},
}

func All() []Registration { return append([]Registration(nil), registrations...) }

func Names() []string {
	names := make([]string, len(registrations))
	for index, registration := range registrations {
		names[index] = registration.Generator.Name()
	}
	return names
}

func Resolve(requested string) (domain.Generator, error) {
	normalized := normalize(requested)
	for _, registration := range registrations {
		if normalize(registration.Generator.Name()) == normalized {
			return registration.Generator, nil
		}
		for _, alias := range registration.Aliases {
			if normalize(alias) == normalized {
				return registration.Generator, nil
			}
		}
	}
	names := Names()
	sort.Strings(names)
	return nil, fmt.Errorf("unknown domain %q (available: %s)", requested, strings.Join(names, ", "))
}

// ResolveAll expands the reserved "all" selection in registration order and
// deduplicates aliases or explicit names that select the same generator.
func ResolveAll(requested []string) ([]domain.Generator, error) {
	result := make([]domain.Generator, 0, len(registrations))
	seen := make(map[string]struct{}, len(registrations))
	add := func(generator domain.Generator) {
		if _, exists := seen[generator.Name()]; exists {
			return
		}
		seen[generator.Name()] = struct{}{}
		result = append(result, generator)
	}
	for _, value := range requested {
		if normalize(value) == "all" {
			for _, registration := range registrations {
				add(registration.Generator)
			}
			continue
		}
		generator, err := Resolve(value)
		if err != nil {
			return nil, err
		}
		add(generator)
	}
	return result, nil
}

func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return value
}
