package parser

import (
	"go/ast"
	"sort"
	"strings"
)

type routeDoc struct {
	summary     string
	description string
	tags        []string
}

func parseRouteDoc(doc *ast.CommentGroup) routeDoc {
	if doc == nil {
		return routeDoc{}
	}
	text := strings.TrimSpace(doc.Text())
	if text == "" {
		return routeDoc{}
	}

	summary := ""
	descLines := make([]string, 0, 4)
	tagSet := map[string]bool{}

	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if v, ok := cutAnnotationValue(line, "summary"); ok {
			summary = strings.TrimSpace(v)
			continue
		}
		if v, ok := cutAnnotationValue(line, "description"); ok {
			descLines = append(descLines, strings.TrimSpace(v))
			continue
		}
		if strings.HasPrefix(lower, "@tag ") || strings.HasPrefix(lower, "@tags ") {
			v := line[strings.IndexByte(line, ' ')+1:]
			for _, item := range splitTags(v) {
				if item != "" {
					tagSet[item] = true
				}
			}
			continue
		}
		if strings.HasPrefix(line, "@") {
			// Skip unhandled swagger directives to avoid polluting description.
			continue
		}
		if summary == "" {
			summary = line
			continue
		}
		descLines = append(descLines, line)
	}

	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)

	return routeDoc{
		summary:     summary,
		description: strings.TrimSpace(strings.Join(descLines, "\n")),
		tags:        tags,
	}
}

func collectTagDefinitions(f *ast.File, out map[string]string) {
	for _, cg := range f.Comments {
		for _, raw := range strings.Split(cg.Text(), "\n") {
			line := strings.TrimSpace(raw)
			if line == "" {
				continue
			}
			lower := strings.ToLower(line)
			if strings.HasPrefix(lower, "@tags ") {
				v := line[strings.IndexByte(line, ' ')+1:]
				for _, name := range splitTags(v) {
					if _, exists := out[name]; !exists {
						out[name] = ""
					}
				}
				continue
			}
			if !strings.HasPrefix(lower, "@tag ") {
				continue
			}
			v := line[strings.IndexByte(line, ' ')+1:]
			name, desc := splitNameAndDesc(strings.TrimSpace(v))
			if name == "" {
				continue
			}
			if _, exists := out[name]; !exists || out[name] == "" {
				out[name] = desc
			}
		}
	}
}

func splitNameAndDesc(s string) (string, string) {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return "", ""
	}
	name := parts[0]
	desc := ""
	if len(parts) > 1 {
		desc = strings.Join(parts[1:], " ")
	}
	return name, strings.TrimSpace(desc)
}

func splitTags(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

type mainDocInfo struct {
	title       string
	version     string
	description string
	basePath    string
	schemes     []string
	host        string
}

func parseMainDocInfo(doc *ast.CommentGroup) mainDocInfo {
	if doc == nil {
		return mainDocInfo{}
	}
	info := mainDocInfo{}
	inSecurityDef := false
	for _, raw := range strings.Split(doc.Text(), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "@securitydefinitions.") {
			inSecurityDef = true
			continue
		}
		if inSecurityDef && strings.HasPrefix(line, "@") {
			if !(strings.HasPrefix(lower, "@in ") || strings.HasPrefix(lower, "@name ") || strings.HasPrefix(lower, "@description ")) {
				inSecurityDef = false
			}
		}
		if v, ok := cutAnnotationValue(line, "title"); ok {
			info.title = strings.TrimSpace(v)
			continue
		}
		if v, ok := cutAnnotationValue(line, "version"); ok {
			info.version = strings.TrimSpace(v)
			continue
		}
		if v, ok := cutAnnotationValue(line, "description"); ok {
			if inSecurityDef {
				continue
			}
			info.description = strings.TrimSpace(v)
			continue
		}
		if v, ok := cutAnnotationValue(line, "basepath"); ok {
			info.basePath = strings.TrimSpace(v)
			continue
		}
		if v, ok := cutAnnotationValue(line, "host"); ok {
			info.host = strings.TrimSpace(v)
			continue
		}
		if v, ok := cutAnnotationValue(line, "schemes"); ok {
			info.schemes = splitTags(strings.ReplaceAll(v, " ", ","))
			continue
		}
		if strings.HasPrefix(line, "@") {
			// Keep current block state for security definition metadata lines.
			continue
		}
		if !strings.HasPrefix(line, "@") {
			inSecurityDef = false
		}
	}
	return info
}

func cutAnnotationValue(line, key string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(line))
	prefix := "@" + strings.ToLower(key) + " "
	if !strings.HasPrefix(lower, prefix) {
		return "", false
	}
	return strings.TrimSpace(line[len(prefix):]), true
}
