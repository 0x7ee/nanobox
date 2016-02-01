// +build windows

package terminal

import "time"

// monitor
func monitor(stdOutFD uintptr) {

	// inform the server what the starting size is
	prevW, prevH := getTTYSize(stdOutFD)
	resizeTTY(prevW, prevH)

	// periodically resize the tty
	for {
		select {
		case <-time.After(time.Millisecond * 250):
			w, h := getTTYSize(stdOutFD)

			if prevW != w || prevH != h {
				resizeTTY(w, h)
			}

			prevW, prevH = w, h
		}
	}
}
