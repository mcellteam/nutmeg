nutmeg
======

nutmeg is a regression test framework for the MCell simulator ([www.mcell.com](http://www.mcell.com)).


Installation
------------

nutmeg is written in go and can be compiled via

<pre><code>go build</code></pre>

in the top level directory. Production versions of nutmeg will be shipped with
precompiled binaries for Linux, Mac OSX, and Windows.

Usage
-----

<pre><code>
nutmeg [option]

Here *option* can be one of

  -c
    clean temporary test data

  -d <test selection>
    show description for selected tests

  -l
    show available test cases

  -m
    number of concurrent test jobs (default: 2)

  -n
    number of concurrent simulation jobs (default: 2)

  -r <test selection>
    run specified tests
</code></pre>

Here, <code>test selection</code> is a comma separated lists of test cases
specified either by their numeric test ID or name (as returned by
<code>nutmeg -l</code>). It is also possible to specify numeric ranges via
<code>i:j</code> with lower and upper limits i and j, respectively.
Please note that the comma separated list may not contain spaces unless the
whole expression is enclosed in double quotes.

Adding New Test Cases
---------------------

To add a new test case, simply create a new and descriptively named subdirectory
in tests/. All files pertaining to this test (including any test output) are
contained in this directory. At a minumum, a test case consists of an MCell
test input file in MDL format and a file called *test_description.json*,
containing the test description in JSON format. A sample test description file
is available in share and can be modofied to fit the tests specific needs (more
details on how to add new tests will follow soon).


Author
------

(C) Markus Dittrich, 2014   National Center for Multiscale Modeling of
                            Biological Systems.
