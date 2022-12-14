// Package sse define Server Side Event feeder for clients
package sse

//
//import (
//	"bufio"
//	"bytes"
//	"io"
//	"strconv"
//
//	"opensvc.com/opensvc/core/event"
//)
//
//const (
//	// maxScanTokenSize defines max len of sse event
//	maxScanTokenSize  = 4096 * 1024
//	initialBufferSize = 4096
//)
//
//// FeedQueue feeds q of parsed server side events from reader r
//func FeedQueue(r io.Reader, q chan<- event.Event) error {
//	var (
//		// fieldSep define the event line field separator
//		fieldSep = []byte{':'}
//
//		// leftTrimValue defines the cut set part of field value to remove
//		leftTrimValue = " "
//
//		dispatchReady bool
//	)
//
//	scanner := bufio.NewScanner(r)
//	scanner.Buffer(make([]byte, initialBufferSize), maxScanTokenSize)
//	ev := event.Event{}
//
//	for scanner.Scan() {
//		line := scanner.Bytes()
//
//		if len(line) > 0 {
//			// not empty line, read fields
//			if fieldName, fieldValue, ok := bytes.Cut(line, fieldSep); ok {
//				fieldValue = bytes.TrimLeft(fieldValue, leftTrimValue)
//				switch string(fieldName) {
//				case "":
//					// line starts with ':' => ignore, it is a comment
//				case "event":
//					dispatchReady = true
//					ev.Kind = string(fieldValue)
//				case "id":
//					if id, err := strconv.ParseUint(string(fieldValue), 10, 64); err != nil {
//						dispatchReady = true
//						ev.ID = id
//					}
//				case "data":
//					dispatchReady = true
//					// concat multiple consecutive data lines
//					ev.Data = append(ev.Data, fieldValue...)
//					ev.Data = append(ev.Data, '\n')
//				case "retry":
//					// drop reconnection field
//				}
//			}
//		} else if dispatchReady {
//			q <- ev
//
//			// reset for next event
//			dispatchReady = false
//			//ev = event.Event{}
//		}
//	}
//	return scanner.Err()
//}
