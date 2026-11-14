package steamhook

import (
	"strings"

	"github.com/ThallesP/macron/internal/failure"
)

type Node struct {
	Key      string
	Value    string
	Children []*Node
	IsObject bool
}

func (n *Node) Child(key string) *Node {
	for _, child := range n.Children {
		if strings.EqualFold(child.Key, key) {
			return child
		}
	}
	return nil
}

func (n *Node) Path(keys ...string) *Node {
	current := n
	for _, key := range keys {
		current = current.Child(key)
		if current == nil {
			return nil
		}
	}
	return current
}

func (n *Node) EnsureObject(keys ...string) *Node {
	current := n
	for _, key := range keys {
		next := current.Child(key)
		if next == nil {
			next = &Node{Key: key, IsObject: true}
			current.Children = append(current.Children, next)
		}
		current = next
	}
	return current
}

func (n *Node) Remove(key string) *Node {
	for i, child := range n.Children {
		if strings.EqualFold(child.Key, key) {
			n.Children = append(n.Children[:i], n.Children[i+1:]...)
			return child
		}
	}
	return nil
}

func (n *Node) Set(child *Node) {
	for i, existing := range n.Children {
		if strings.EqualFold(existing.Key, child.Key) {
			n.Children[i] = child
			return
		}
	}
	n.Children = append(n.Children, child)
}

func ParseVDF(text string) (*Node, error) {
	tokens, err := tokenizeVDF(text)
	if err != nil {
		return nil, err
	}

	root := &Node{IsObject: true}
	stack := []*Node{root}
	for i := 0; i < len(tokens); i++ {
		current := stack[len(stack)-1]
		token := tokens[i]
		if token.isClose {
			if len(stack) == 1 {
				return nil, failure.Errorf(failure.CodeInvalidVDF, "unexpected } in VDF")
			}
			stack = stack[:len(stack)-1]
			continue
		}
		if token.isOpen {
			return nil, failure.Errorf(failure.CodeInvalidVDF, "unexpected { in VDF")
		}
		if i+1 == len(tokens) {
			return nil, failure.Errorf(failure.CodeInvalidVDF, "key %q has no value", token.text)
		}

		next := tokens[i+1]
		i++
		if next.isClose {
			return nil, failure.Errorf(failure.CodeInvalidVDF, "key %q has no value", token.text)
		}
		if next.isOpen {
			child := &Node{Key: token.text, IsObject: true}
			current.Children = append(current.Children, child)
			stack = append(stack, child)
			continue
		}
		current.Children = append(current.Children, &Node{Key: token.text, Value: next.text})
	}
	if len(stack) != 1 {
		return nil, failure.Errorf(failure.CodeInvalidVDF, "VDF ends inside %q", stack[len(stack)-1].Key)
	}
	return root, nil
}

type vdfToken struct {
	text    string
	isOpen  bool
	isClose bool
}

func tokenizeVDF(text string) ([]vdfToken, error) {
	var tokens []vdfToken
	for i := 0; i < len(text); i++ {
		switch c := text[i]; {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
		case c == '{':
			tokens = append(tokens, vdfToken{isOpen: true})
		case c == '}':
			tokens = append(tokens, vdfToken{isClose: true})
		case c == '/' && i+1 < len(text) && text[i+1] == '/':
			for i < len(text) && text[i] != '\n' {
				i++
			}
		case c == '"':
			value, end, err := readQuoted(text, i+1)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, vdfToken{text: value})
			i = end
		default:
			end := i
			for end < len(text) && !strings.ContainsRune(" \t\r\n{}\"", rune(text[end])) {
				end++
			}
			tokens = append(tokens, vdfToken{text: text[i:end]})
			i = end - 1
		}
	}
	return tokens, nil
}

func readQuoted(text string, start int) (string, int, error) {
	var value strings.Builder
	for i := start; i < len(text); i++ {
		c := text[i]
		if c == '"' {
			return value.String(), i, nil
		}
		if c == '\\' && i+1 < len(text) {
			i++
			value.WriteByte(unescapeVDF(text[i]))
			continue
		}
		value.WriteByte(c)
	}
	return "", 0, failure.Errorf(failure.CodeInvalidVDF, "unterminated string in VDF")
}

func unescapeVDF(c byte) byte {
	switch c {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	default:
		return c
	}
}

func (n *Node) String() string {
	var out strings.Builder
	for _, child := range n.Children {
		writeVDF(&out, child, 0)
	}
	return out.String()
}

func writeVDF(out *strings.Builder, n *Node, depth int) {
	indent := strings.Repeat("\t", depth)
	if !n.IsObject {
		out.WriteString(indent + quoteVDF(n.Key) + "\t\t" + quoteVDF(n.Value) + "\n")
		return
	}

	out.WriteString(indent + quoteVDF(n.Key) + "\n" + indent + "{\n")
	for _, child := range n.Children {
		writeVDF(out, child, depth+1)
	}
	out.WriteString(indent + "}\n")
}

func quoteVDF(text string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`).Replace(text)
	return `"` + escaped + `"`
}
