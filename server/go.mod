module github.com/kind84/polygo

go 1.12

require (
	cloud.google.com/go v0.43.0
	github.com/go-redis/redis v6.15.6+incompatible
	github.com/golang/snappy v0.0.1 // indirect
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kind84/polygo/storyblok v0.0.0
	github.com/kind84/polygo/translator v0.0.0
	github.com/pelletier/go-toml v1.6.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.5.0
	github.com/tidwall/pretty v1.0.0 // indirect
	github.com/xdg/scram v0.0.0-20180814205039-7eeb5667e42c // indirect
	github.com/xdg/stringprep v1.0.0 // indirect
	go.mongodb.org/mongo-driver v1.1.2 // indirect
	golang.org/x/sys v0.0.0-20191029155521-f43be2a4598c // indirect
	golang.org/x/text v0.3.2
	google.golang.org/api v0.7.0
)

replace (
	github.com/kind84/polygo/storyblok v0.0.0 => ../storyblok
	github.com/kind84/polygo/translator v0.0.0 => ../translator
)
