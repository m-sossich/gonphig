## Gonphig usage

### Read configurations from file
```
type Conf struct {
    Field string 
    FieldOther string 
}
...
var conf ProfileConfig
if err := config.ReadConfiguration("path/to/file", &conf); err != nil {
	log.Fatal(err)
}
```

### Read configuration from flags

To overwrite the configuration value with a flag, just indicate the name of the flag in the configuration struct, flag-usage is optional

```
type Conf struct {
    Field string    `flag:"flag-for-field" flag-usage:"not important"`
}

...
var conf ProfileConfig
if err := config.ReadConfiguration("path/to/file", &conf); err != nil {
	log.Fatal(err)


```

Gonphig will take care of the flag declaration and the `flag.Parse()` for you when it reads the configuration

### Read configuration from ENV variables

To overwrite the configuration value with a env variable, just indicate the name of the flag in the configuration struct

```
type Conf struct {
   Field string     `env:"env_variable_to_read"`
}

...
var conf ProfileConfig
if err := config.ReadConfiguration("path/to/file", &conf); err != nil {
	log.Fatal(err)
}

```
# Licence
Original work was done under the [dcl-viper](https://github.com/decentraland/dcl-viper) name. Part of the [Decentraland](https://decentraland.org/) project
