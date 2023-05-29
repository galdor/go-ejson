package main

import (
	"encoding/json"
	"os"

	jsonpointer "github.com/galdor/go-ejson"
	"github.com/galdor/go-program"
)

func main() {
	var c *program.Command

	p := program.NewProgram("json-pointer",
		"example of JSON pointer manipulation")

	c = p.AddCommand("find",
		"extract and print the json value referenced by a pointer", cmdFind)
	c.AddArgument("pointer", "the json pointer")
	c.AddOptionalArgument("path", "the file containing the json document")

	p.ParseCommandLine()
	p.Run()
}

func cmdFind(p *program.Program) {
	pointerString := p.ArgumentValue("pointer")
	var pointer jsonpointer.Pointer
	if err := pointer.Parse(pointerString); err != nil {
		p.Fatal("invalid json pointer: %v", err)
	}

	filePath := p.ArgumentValue("path")
	var file *os.File
	if filePath == "" {
		file = os.Stdin
	} else {
		var err error
		file, err = os.Open(filePath)
		if err != nil {
			p.Fatal("cannot open %q: %v", filePath, err)
		}
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	var document interface{}
	if err := decoder.Decode(&document); err != nil {
		p.Fatal("cannot parse json data: %v", err)
	}

	value := pointer.Find(document)

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(value); err != nil {
		p.Fatal("cannot serialize json value: %v", err)
	}
}
