# Brahe

This is a really simple command-line app that I wrote to compare the contents of multiple supposedly equal directories. I have successfully used it to compare whole disks containing 100k+ files.

You define a source directory (the source of truth) and then any number of target directories. It will then make sure the directory/file hierarchy matches and file contents match based on [BLAKE2b](https://blake2.net/), which was chosen for its speed while still being cryptographically secure. Right now it works in haystack mode, which means it only checks if the target directories contain everything that the source contains. The target directories can also contain any number of other files and the program won't care.

# Usage

```
brahe [options] [source] [target1] .. [targetN]
  -find-gaps string
        Value of 'IMG_/4:14-155/.JPG' searches for gaps in sequence of IMG_0014.JPG .. IMG_0155.JPG. Value of '/0:1-13/.txt' seeks 1.txt .. 13.txt.
  -no-data
        Don't compare the file contents.
  -system-names
        Also check system names like $RECYCLE.BIN;System Volume Information;found.000;Thumbs.db.
```

# Project status

This project is not actively maintained, however feel free to send bug reports or pull requests.

# About

© Copyright 2016 - 2019 [Kaur Kuut](https://www.kaurkuut.com)

Licensed under the [Apache 2.0 license](http://www.apache.org/licenses/LICENSE-2.0)