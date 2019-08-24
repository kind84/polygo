module github.com/kind84/polygo/translator

go 1.12

require (
	cloud.google.com/go v0.43.0
	github.com/go-redis/redis v6.15.2+incompatible
	github.com/kind84/polygo/storyblok v0.0.0
	github.com/spf13/viper v1.4.0
	golang.org/x/text v0.3.2
	golang.org/x/tools/gopls v0.1.3 // indirect
)

replace github.com/kind84/polygo/storyblok v0.0.0 => ../storyblok
