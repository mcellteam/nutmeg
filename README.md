nutmeg
======

nutmeg is a regression test framework for the MCell simulator ([www.mcell.org](http://www.mcell.org)).

Installation
------------

The nutmeg test framework is written in go. If you don't already use go, you'll
probably need to set your GOPATH, which you can do like this:

    export GOPATH=$HOME/go

Note: feel free to use some other path than `$HOME/go`.

To pull in nutmeg's dependencies, change into the top-level nutmeg directory
and type this:

    go get -u all

Finally, to build nutmeg, enter this:

    go build

Production versions of nutmeg will be shipped with precompiled binaries for
Linux, Mac OSX, and Windows.

nutmeg expects a config file in TOML format called **nutmeg.conf** in the
directory where the binary is located. Currently, this file only contains
information on the path of the MCell executable and the location of
the *tests/* directory. A sample *nutmeg.conf* is available in the *share/*
subdirectory.


Usage
-----

<pre><code>
nutmeg [option]

Here [option] can be one of

  -c
    clean temporary test data

  -d test_selection
    show description for selected tests

  -l
    show available test cases

  -L
    show available test categories

  -m
    number of concurrent test jobs (default: 2)

  -n
    number of concurrent simulation jobs (default: 2)

  -r test_selection
    run specified tests (i, i:j, 'all')

  -R test_category
    run all the tests in a given category (e.g. reactions, parser)

</code></pre>

Here, `test_selection` is a comma separated lists of test cases specified
either by their numeric test ID or their name (as returned by `nutmeg -l`). It
is also possible to specify numeric ranges via `i:j` with lower and upper
limits i and j, respectively. The keyword `all` triggers testing of all
available cases.  Please note that the comma separated list of
`test_selection`s may not contain spaces unless the whole expression is
enclosed in double quotes.  Concurrent simulation jobs will each run as
separate MCell processes. Thus, to optimize throughput n should be chosen as
large as possible but not exceed the physical number of cores on the test
machine.

Examples
--------

Run all the tests:

    ./nutmeg -r all

Run the `count_enclosed` and `all_enclosed` test:

    ./nutmeg -r count_enclosed,all_enclosed

Run tests eighty through eighty-four (the numbers correspond to those shown by
`nutmeg -l`):

    ./nutmeg -r 80:85

Run all the dynamic geometry tests:

    ./nutmeg -R "dynamic geometry"

Adding New Test Cases
---------------------

To add a new test case, simply create a new and descriptively named
subdirectory in **tests/**. All files pertaining to this test (including any
test output) are contained in this directory. At a minumum, a test case
consists of an MCell test input file in MDL format and a file called
**test_description.toml**, containing the description of this particular test
case in [TOML format](https://github.com/toml-lang/toml). A sample test
description file is available in **share/** and can be modified to fit the
specific needs of new test cases (more details on how to add new tests will
follow soon).


Author
------

(C) Markus Dittrich, 2014   [National Center for Multiscale Modeling of
                            Biological Systems](http://www.mmbios.org).
