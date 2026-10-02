package cli

import (
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// guideFS holds the agent guide, one Markdown file per topic, so it matches the binary (DECISIONS 276).
//
//go:embed guide/*.md
var guideFS embed.FS

// runGuide is `canon guide [topic]` (CLI.md §3.17): the index, or one topic; an unknown topic lists them (exit 2).
func runGuide(inv *invocation) int {
	if len(inv.args) > 1 {
		return inv.fail(fmt.Errorf(fmtArgs, cmdGuide, errAtMostOneArg))
	}
	topic := guideIndex
	if len(inv.args) == 1 {
		topic = inv.args[0]
	}
	topics, err := guideTopics()
	if err != nil {
		return inv.fail(err)
	}
	if !slices.Contains(topics, topic) {
		return inv.fail(fmt.Errorf(fmtUnknownTopic, cmdGuide, topic, errUnknownTopic, strings.Join(topics, listSep)))
	}
	text, err := guideFS.ReadFile(guideDir + pathSep + topic + guideExt)
	if err != nil {
		return inv.fail(fmt.Errorf(fmtWrap, err))
	}
	if err := writeText(inv.env.Stdout, string(text)); err != nil {
		return inv.fail(err)
	}
	return exitOK
}

// guideTopics are the embedded topics, in byte order of their names.
func guideTopics() ([]string, error) {
	entries, err := fs.ReadDir(guideFS, guideDir)
	if err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, strings.TrimSuffix(e.Name(), guideExt))
	}
	return out, nil
}
