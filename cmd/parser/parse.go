package main

import (
	"fmt"
	"os"
	"text/template/parse"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("failed to parse: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	f, err := os.ReadFile("./stacks/dhis2-core/helmfile.yaml")
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}
	t := parse.New("")
	// TODO(ivo) there was something about comments also rendering env vars? so I assume we should
	// also parse them, right?
	t.Mode = parse.SkipFuncCheck | parse.ParseComments

	t1, err := t.Parse(string(f), "", "", map[string]*parse.Tree{})
	if err != nil {
		return fmt.Errorf("failed to parse file: %v", err)
	}

	envVars := make(map[string]parse.Node)
	// TODO(ivo) walk the tree correctly
	for _, n := range t1.Root.Nodes {
		switch n.Type() {
		case parse.NodeAction:
			actionNode := n.(*parse.ActionNode)
			for _, commandNode := range actionNode.Pipe.Cmds {
				// TODO(ivo) I don't know the syntax of text templates well. If I check the arg to
				// be an identifier of `requiredEnv` or `env` then I am only interested in the next
				// or next 2 (env name and default) args
				// not sure how a | will change the tree

				// TODO(ivo) create proper loop
				if len(commandNode.Args) > 1 && commandNode.Args[0].Type() == parse.NodeIdentifier {
					fmt.Printf("command %#v with arg %#v\n", commandNode.Args[0], commandNode.Args[1])
					identifier := commandNode.Args[0].(*parse.IdentifierNode)
					if identifier.Ident == "requiredEnv" {
						stringNode, ok := commandNode.Args[1].(*parse.StringNode)
						if ok {
							envVars[stringNode.Text] = nil
						}
					} else if identifier.Ident == "env" {
						stringNode, ok := commandNode.Args[1].(*parse.StringNode)
						if ok {
							// TODO(ivo) get default value
							envVars[stringNode.Text] = nil
						}
					}
				}
			}
		}
	}
	fmt.Println()
	fmt.Printf("env vars: %v", envVars)

	return nil
}
