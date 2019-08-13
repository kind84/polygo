module github.com/kind84/polygo/translator

go 1.12

require (
	cloud.google.com/go v0.43.0
	github.com/go-redis/redis v6.15.2+incompatible
	github.com/kind84/polygo/storyblok v0.0.0
	github.com/mdempsky/gocode v0.0.0-20190203001940-7fb65232883f // indirect
	github.com/spf13/viper v1.4.0
	golang.org/x/text v0.3.2
	golang.org/x/tools v0.0.0-20190809145639-6d4652c779c4 // indirect
	google.golang.org/api v0.7.0
)

replace github.com/kind84/polygo/storyblok v0.0.0 => ../storyblok
