module github.com/kind84/polygo

go 1.12

require (
	cloud.google.com/go v0.43.0
	github.com/go-redis/redis v6.15.2+incompatible
	github.com/julienschmidt/httprouter v1.2.0
	github.com/kind84/polygo/storyblok v0.0.0
	github.com/kind84/polygo/translator v0.0.0
	github.com/spf13/viper v1.4.0
	golang.org/x/text v0.3.2
	google.golang.org/api v0.7.0
)

replace (
	github.com/kind84/polygo/storyblok v0.0.0 => ../storyblok
	github.com/kind84/polygo/translator v0.0.0 => ../translator
)
