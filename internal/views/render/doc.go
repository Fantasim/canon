// Package render renders the view templates of the view model (spec/VIEWMODEL.md 9.2): titles
// and subtitles of search rows, `show` lines and view-named methods' values (L21), refs by their
// target's title (S8), enum members by their label, search terms and previews (S2), in the
// source language and in each translation (X6), a value's magic names given where it is placed
// (3.4). The view expressions are evaluated by an Evaluator the caller provides.
package render
