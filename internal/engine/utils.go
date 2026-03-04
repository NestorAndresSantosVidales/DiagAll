package engine

import (
	"io/ioutil"
	"strings"
)

// ListSessions returns a list of session IDs found in the directory.
func ListSessions(dir string) ([]string, error) {
	var sessions []string

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		// start in current dir
		files, err = ioutil.ReadDir(".")
		if err != nil {
			return nil, err
		}
	}

	for _, f := range files {
		if !f.IsDir() && strings.HasPrefix(f.Name(), "session_") && strings.HasSuffix(f.Name(), ".json") {
			sessions = append(sessions, f.Name())
		}
	}
	return sessions, nil
}
