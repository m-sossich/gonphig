# <p align="center"><img src="https://raw.githubusercontent.com/m-sossich/gonphig/master/.github/logo.png" width="450"></p>

<p align="right"> 

![Go](https://github.com/m-sossich/gonphig/workflows/Go/badge.svg?branch=master) 

</p>

## What is this for?
Read Configurations from different sources

## How to use it

### Read configuration from flags

To load flags into your configuration struct, just indicate the name of the flag with the `flag` tag, `flag-usage` is optional

```
type Conf struct {
    Field string    `flag:"flag-for-field" flag-usage:"not important"`
}

...
var conf ProfileConfig
if err := gonphig.ReadConfig(&conf); err != nil {
	log.Fatal(err)
```

Gonphig will take care of the flag declaration and the `flag.Parse()` for you when it reads the configuration

### Read configuration from ENV variables

To load env-var into your configuration struct, just indicate the name of the variable with the `env` tag

```
type Conf struct {
   Field string     `env:"env_variable_to_read"`
}

...
var conf ProfileConfig
if err := gonphig.ReadConfig(&conf); err != nil {
	log.Fatal(err)
}
```

### Assign default values

You can add a default value to your configuration using the `default` tag

```
type Conf struct {
   Field string     `default:"someValue"`
   AnInt int        `default:"42"`
}
```

### Read configurations from a YAML file

The values provided on the file will be taken as the default values

```
type Conf struct {
    Field string 
}
...
var conf ProfileConfig
if err := gonphig.ReadFromFile("path/to/file.yml", &conf); err != nil {
	log.Fatal(err)
}
```

If you want to rename a field in the yaml file and map it into your config struct, you can use the `yaml` tag

```
type Conf struct {
    NotTheSameName string `yaml:"field"`
}
```

### Validation

It is possible to add validations over the configuration values using the `validate` tag

```
type Conf struct {
   Field string `env:"env_variable_to_read" validate:"required"`
}
```

### Configuration hierarchy
1. Configuration read from `flags` 
2. Configuration read from `env-var`
3. Configuration coming from the `default` tag
4. Configuration read from the `yaml` file

## External resources
### YAML
* [SPEC](https://yaml.org/spec/1.2/spec.html)
* To read the YAML file we are using an external library. All information can be found [here](https://github.com/go-yaml/yaml)

### Validations
* To perform validations on the configuration struct, We are using an external library. All information can be found [here](https://github.com/go-playground/validator/tree/v9)