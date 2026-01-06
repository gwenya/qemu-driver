package config

import (
	"fmt"
	"io"
	"strings"
)

type Builder struct {
	sections []Section
}

type Section struct {
	Name    string
	Entries map[string]string
}

func (c *Builder) AddSection(section Section) {
	c.sections = append(c.sections, section)
}

func (c *Builder) String() string {
	b := &strings.Builder{}
	// writing to a string builder never returns an error
	_, _ = c.WriteTo(b)
	return b.String()
}

func (c *Builder) WriteTo(w io.Writer) (int64, error) {
	total := int64(0)
	for _, section := range c.sections {
		n, err := fmt.Fprintf(w, "[%s]\n", section.Name)
		total += int64(n)
		if err != nil {
			return total, err
		}

		for key, value := range section.Entries {
			n, err := fmt.Fprintf(w, "%s = \"%s\"\n", key, value)
			total += int64(n)
			if err != nil {
				return total, err
			}
		}

		n, err = fmt.Fprint(w, "\n")
		total += int64(n)
		if err != nil {
			return total, err
		}
	}

	return total, nil
}
