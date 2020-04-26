# gopolls
A library for Go (Golang) to support different polling procedures.

## About This Library
This library supports three different kind of polls: Normal polls (Aye, No, Abstention,),
Median Polls (the highest value in a sequence of numbers that has a majority wins) and
Schulze Polls with the [Schulze method](https://en.wikipedia.org/wiki/Schulze_method).

Note: There is no extensive documentation / usage examples yet.
This library is still in an early stage and likely to change with changes that are
not backwards compatible. So use it with care!

For now you can hav a look at the [docs on godocs](https://godoc.org/github.com/FabianWe/gopolls).

## Simple App
This project comes with a demo app that builds a simple (and not good / production ready) webserver.
The code can be found in [cmd/poll](cmd/poll).

There are binary [releases](https://github.com/FabianWe/gopolls/releases) of this app for linux, mac and Windows.
Just download the `.zip` file and execute the binary.

As already mentioned there is no documentation yet, so usage examples will follow once the API
is more or less stable.

The [Wiki](https://github.com/FabianWe/gopolls/wiki) will most likely contain this documentation.

## License
Copyright 2020 Fabian Wenzelmann <fabianwen@posteo.eu>

[Licensed under the Apache License, Version 2.0](LICENSE)

### Third-Party Licenses

This tool is built with only the [Golang](https://golang.org) standard library ([License](https://golang.org/LICENSE)).
It uses however [pure-css](https://purecss.io/) (contained in the distributions of the demo app).
pure-css is licensed under the [Yahoo BSD License](https://github.com/pure-css/pure-site/blob/master/LICENSE.md) 
License.

It also uses a template from the [pure-css Layouts page](https://purecss.io/layouts) (also contained
in this distribution).

All these files are contained [here](cmd/poll/static/pure-release-1.0.1) and [here](cmd/poll/static/layout) and are not
covered but my copyright but the copyrights mentioned above.
