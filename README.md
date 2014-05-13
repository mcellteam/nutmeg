nutmeg
======

nutmeg is a regression test framework for the MCell simulator ([www.mcell.org](http://www.mcell.org)).


Installation
------------

nutmeg is written in go and can be compiled via

<pre><code>go build</code></pre>

in the top level directory. Production versions of nutmeg will be shipped with
precompiled binaries for Linux, Mac OSX, and Windows.

nutmeg expects a config file in JSON format called **nutmeg.conf** in the
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

  -m
    number of concurrent test jobs (default: 2)

  -n
    number of concurrent simulation jobs (default: 2)

  -r test_selection
    run specified tests
</code></pre>

Here, <code>test selection</code> is a comma separated lists of test cases
specified either by their numeric test ID or their name (as returned by
<code>nutmeg -l</code>). It is also possible to specify numeric ranges via
<code>i:j</code> with lower and upper limits i and j, respectively. The
keyword <code>all</code> triggers testing of all available cases.
Please note that the comma separated list of test_selections may not contain
spaces unless the whole expression is enclosed in double quotes.
Concurrent simulation jobs will each run as separate MCell processes. Thus, to
optimize throughput n should be chosen as large as possible but not exceed the
physical number of cores on the test machine.

Adding New Test Cases
---------------------

To add a new test case, simply create a new and descriptively named subdirectory
in **tests/**. All files pertaining to this test (including any test output) are
contained in this directory. At a minumum, a test case consists of an MCell
test input file in MDL format and a file called **test_description.json**,
containing the description of this particular test case in JSON format.
A sample test description file is available in **share/** and can be modified
to fit the specific needs of new test cases (more details on how to add new
tests will follow soon).


Author
------

(C) Markus Dittrich, 2014   [National Center for Multiscale Modeling of
                            Biological Systems](www.mmbios.org).
