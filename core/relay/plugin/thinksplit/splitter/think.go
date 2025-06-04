package splitter

import "github.com/labring/aiproxy/core/common/conv"

const (
	NThinkHead = "\n<think>\n"
	ThinkHead  = "<think>\n"
	ThinkTail  = "</think>\n"
)

var (
	nthinkHeadBytes = conv.StringToBytes(NThinkHead)
	thinkHeadBytes  = conv.StringToBytes(ThinkHead)
	thinkTailBytes  = conv.StringToBytes(ThinkTail)
)

func NewThinkSplitter() *Splitter {
	return NewSplitter([][]byte{nthinkHeadBytes, thinkHeadBytes}, [][]byte{thinkTailBytes})
}
