# dcl-viper
Viper extension to read configurations

Using [Viper](https://github.com/spf13/viper) in the core of this reading functionality. we should be able to read from a config file, a env variable or a flag.

## Viper normal usage

You can use viper to load a config file to a struct containing all configurations example:
```
type Conf struct {
    Filed string
}
```
If you read from a yml containing a `field: 'something'` entry,  Viper will load the value to the field attribute. 

It is also possible to bind a env var to a key, or to overwrite a key with a given value, but you need to bind each attribute manually, the same goes for overwriting it with a flag. Plus you need to know the flags before hand and this hole thing becomes annoying and repetitive very fast.

To bind a env variable you need to do something like this:
```
viper.ReadInConfig()
viper.GetViper().BindEnv("key.in.the.struct", "ENV_VAR_NAME")
```
This is for each field, in each project

### Reading from flags

If you want to read from a flag you will need to do:
```
flagValue := flag.String("a-flag", "the default value", "usage example or somethin")
flag.Parse()
...
viper.ReadInConfig()
viper.Set("key.in.the.struct", flagValue)
```
This is for each field, in each project

## DCL-Viper usage

### Read configurations from file
```
type Conf struct {
    Filed string 
    FiledOther string 
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
    Filed string          `overwrite-flag:"flag-for-field" flag-usage:"not important"`
}

...
var conf ProfileConfig
if err := config.ReadConfiguration("path/to/file", &conf); err != nil {
	log.Fatal(err)


```

Dcl-viper will take care of the flag declaration and the `flag.Parse()` for you when it reads the configuration

### Read configuration from ENV variables

To overwrite the configuration value with a env variable, just indicate the name of the flag in the configuration struct

```
type Conf struct {
   FiledOther string `overwrite-env:"env_variable_to_read"`
}

...
var conf ProfileConfig
if err := config.ReadConfiguration("path/to/file", &conf); err != nil {
	log.Fatal(err)


```
