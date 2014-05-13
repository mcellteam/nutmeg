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
nutmeg <option>

Here options can be one of

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

Here, a test selection is a comma separated lists of tests specified either
by their numeric test ID or name. It is also possible to specify numeric
ranges via *i:j* with lower and upper limits i and j, respectively. Please note
that the comma separated list may not contain spaces unless the whole expression
is enclosed in double quotes.
