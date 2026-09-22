package config

// Keymap is the built-in table from spec §6; config.keymap overrides single
// entries. The page asks for the merged table at startup.
var Keymap = map[string]string{
	"next_hunk": "j", "prev_hunk": "k",
	"next_file": "J", "prev_file": "K",
	"next_comment": "n", "prev_comment": "p",
	"tab_code": "1", "tab_diagrams": "2", "tab_review": "3",
	"comment": "c", "comment_overall": "C",
	"send": "s", "toggle_resolved": "r", "filter_files": "f",
	"toggle_nav": "[", "toggle_aside": "]",
	"theme": "t", "help": "?", "escape": "Escape",
}

func (c Config) MergedKeymap() map[string]string {
	out := make(map[string]string, len(Keymap))
	for k, v := range Keymap {
		out[k] = v
	}
	for k, v := range c.Keymap {
		if _, known := Keymap[k]; known {
			out[k] = v
		}
	}
	return out
}
