package utility

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWindower(t *testing.T) {
	type TestCase struct {
		name     string
		b        *Windower[byte]
		input    []byte
		expected [][]byte
	}

	tcs := []TestCase{
		{
			name:  "step smaller than width",
			b:     NewWindower[byte](5, 2),
			input: []byte("hello there"),
			expected: [][]byte{
				append([]byte{0, 0, 0}, []byte("he")...),
				append([]byte{0}, []byte("hell")...),
				[]byte("ello "),
				[]byte("lo th"),
				[]byte(" ther"),
				append([]byte("here"), 0),
				append([]byte("re"), 0, 0, 0),
				{0, 0, 0, 0, 0},
			},
		},
		{
			name:  "step larger than width",
			b:     NewWindower[byte](3, 5),
			input: []byte("a surprise to be sure"),
			expected: [][]byte{
				[]byte("sur"),
				[]byte("ise"),
				[]byte("o b"),
				[]byte("sur"),
			},
		},
		{
			name:  "step equal to width",
			b:     NewWindower[byte](4, 4),
			input: []byte("i am the senate"),
			expected: [][]byte{
				[]byte("i am"),
				[]byte(" the"),
				[]byte(" sen"),
				append([]byte("ate"), 0),
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			for chunkSize := 1; chunkSize <= 3; chunkSize++ {
				inCh := make(chan []byte, 100)
				outCh := make(chan []byte, 100)

				for i := 0; i < len(tc.input); i += chunkSize {
					end := i + chunkSize
					if end > len(tc.input) {
						end = len(tc.input)
					}

					chunk := tc.input[i:end]
					inCh <- chunk
				}

				close(inCh)
				tc.b.Do(inCh, outCh)

				var i int
				for o := range outCh {
					assert.Less(t, i, len(tc.expected), "too many windows in output")
					assert.Equal(t, tc.expected[i], o, "incorrect window content")
					i++
				}
				assert.Equal(t, i, len(tc.expected), "too few windows in output")
			}
		})
	}
}
