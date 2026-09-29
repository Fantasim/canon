// Package jsonsrc reads JSON sources into trees located by byte spans and RFC 6901 pointers
// (LOD-02), and prints them in the canonical layout of JSON sources (FMT-02). It reports the
// syntax and repeated-key findings; an encoding error is returned for load to report. For the
// edit API's minimal writes, Rewrite applies Set, Insert and Remove edits named by pointer
// (Node.Find) keeping every other byte, and Place gives a new key's position.
package jsonsrc
