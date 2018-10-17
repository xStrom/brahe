# Compare

This is a really simple command-line app that I wrote to compare the contents of three supposedly equal directories containing 10k files (250 GB).

It supports N different directories, which are defined in the source. It will then make sure the directory/file hierarchy matches and file contents match based on SHA-1.
SHA-1 was chosen for speed, but be aware that it's not cryptographically secure, so something stronger should be used if you're checking against more than standard storage failure.

The `haysearch` option means that the first directory can be a subset of the others.

# Project status

This project is not actively maintained, however feel free to send bug reports or pull requests.

# About

© Copyright 2016 - 2018 [Kaur Kuut](https://www.kaurkuut.com)

Licensed under the [Apache 2.0 license](http://www.apache.org/licenses/LICENSE-2.0)