package stack

import (
	"fmt"
	"text/template/parse"
)

// TODO(ivo) pass in the system parameters as well
// newTmpl creates a stack template of given name and with given stackParameters. stackParameters
// will be excluded from optional and required parameters when executing parse.
func newTmpl(name string, stackParameters map[string]struct{}) *tmpl {
	return &tmpl{
		name:            name,
		stackParameters: stackParameters,
		systemParameters: map[string]struct{}{
			"INSTANCE_ID":        {},
			"INSTANCE_NAME":      {},
			"INSTANCE_HOSTNAME":  {},
			"INSTANCE_NAMESPACE": {},
			"IM_ACCESS_TOKEN":    {},
		},
	}
}

// tmpl represents a stack template used to create the environment for running a stacks helmfile
// commands.
type tmpl struct {
	name             string
	requiredEnvs     map[string]struct{}
	envs             map[string]any
	stackParameters  map[string]struct{}
	systemParameters map[string]struct{}
}

func (t *tmpl) parse(in string) error {
	t1 := parse.New(t.name)
	t1.Mode = parse.SkipFuncCheck | parse.ParseComments
	tree, err := t1.Parse(in, "", "", map[string]*parse.Tree{})
	if err != nil {
		return fmt.Errorf("error parsing text template: %v", err)
	}

	// TODO(ivo) read text/template pkg/specs
	// TODO(ivo) implement collecting env vars
	_ = tree

	return nil
}

// walkTree walks given tree node and collects environment variables in template.
func walkTree(node parse.Node, t *tmpl) error {
	// TODO(ivo) comment is considered empty. Double-check that we want to parse comments
	if parse.IsEmptyTree(node) && node.Type() != parse.NodeComment {
		return nil
	}

	switch n := node.(type) {
	case *parse.ListNode:
		for _, nodeElement := range n.Nodes {
			// TODO(ivo) capture bug?
			err := walkTree(nodeElement, t)
			if err != nil {
				return err
			}
		}
	case *parse.ActionNode:
		for _, commandNode := range n.Pipe.Cmds {
			// TODO(ivo) add test for a template that misses an arg to requiredEnv/env as this would
			// cause an error at runtime (when applying the helmfile) otherwise
			if len(commandNode.Args) >= 1 && commandNode.Args[0].Type() == parse.NodeIdentifier {
				identifier := commandNode.Args[0].(*parse.IdentifierNode)
				if identifier.Ident == "requiredEnv" {
					stringNode, ok := commandNode.Args[1].(*parse.StringNode)
					if ok {
						t.requiredEnvs[stringNode.Text] = struct{}{}
					}
				} else if identifier.Ident == "env" {
					stringNode, ok := commandNode.Args[1].(*parse.StringNode)
					if ok {
						// TODO(ivo) get default value
						t.envs[stringNode.Text] = nil
					}
				}
			}
		}
	}
	return nil
}
