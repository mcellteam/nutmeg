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
    
