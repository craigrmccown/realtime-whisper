module github.com/craigrmccown/windowed-markov

go 1.21.7

require (
	github.com/go-audio/audio v1.0.0
	github.com/gordonklaus/portaudio v0.0.0-20230709114228-aafa478834f5
	github.com/lithammer/fuzzysearch v1.1.8
	github.com/mb-14/gomarkov v0.0.0-20231120193207-9cbdc8df67a8
	golang.org/x/sync v0.1.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-audio/riff v1.0.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require (
	github.com/go-audio/wav v1.1.0
	github.com/stretchr/testify v1.9.0
	golang.org/x/text v0.9.0 // indirect
)

// Clone https://github.com/craigrmccown/gomarkov.git alongside this repository.
// It includes a hack to receive deterministic markov chain predictions
replace github.com/mb-14/gomarkov => ../gomarkov
