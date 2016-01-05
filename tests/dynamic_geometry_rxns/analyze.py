#!/usr/bin/env python

# Run this file to generate the min and max counts used in
# test_description.json

import os
import numpy as np


mol_counts = None
mol_means = None
mol_stds = None
files = os.listdir(os.getcwd())   # build a list of reaction data file names
file_prefix = "counts"
files = [f for f in files if f.startswith(file_prefix)]
files.sort()                      # sort that list alphabetically

with open("counts.00001.txt") as f:
    lines = f.readlines()
    first_line = lines[1].split()
    observables_num = len(first_line)

for obs_num in range(1, observables_num):
    for f in files:                    # iterate over the list of file names
        rxn_data = np.genfromtxt("./%s" % f, dtype=float)
        rxn_data = rxn_data[:, obs_num]
        if mol_counts is None:
            mol_counts = rxn_data
        else:
            # build up 2d array of molecule counts (one col/seed)
            mol_counts = np.column_stack((mol_counts, rxn_data))

    mol_mean = mol_counts.mean(axis=1)  # take the mean of the rows
    if mol_means is None:
        mol_means = mol_mean
    else:
        mol_means = np.column_stack((mol_means, mol_mean))

    mol_std = mol_counts.std(axis=1)  # take the std of the rows
    if mol_stds is None:
        mol_stds = mol_std
    else:
        mol_stds = np.column_stack((mol_stds, mol_std))
    
    mol_counts = None

with open('means.dat', 'w') as f:
   np.savetxt(f, mol_means)

with open('stds.dat', 'w') as f:
   np.savetxt(f, mol_stds)

minmax_str = """
       {
           "testType": "COUNT_MINMAX",
           "dataFile": "counts.txt",
           "haveHeader": true,
           "minTime": %g,
           "maxTime": %g,
           "countMinimum": [
%s
           ],
           "countMaximum": [
%s
           ]
       },"""

dt = 1e-5
with open("test_description_partial.json", "w") as test_descript_file, \
        open('means.dat', 'r') as means_file, \
        open('stds.dat', 'r') as stds_file:
    means_lines = means_file.readlines()
    stds_lines = stds_file.readlines()
    for idx, means, stds in zip(range(len(means_lines)), means_lines, stds_lines):
        # Run checks every 10 iterations. Skip first.
        if (((idx % 10) == 0) and idx != 0):
            means = means.split()
            stds = stds.split()
            cmins = "               "
            cmaxes = "               "
            cmins += ",\n               ".join(str(int(float(m)-float(s)*3)) for m, s in zip(means, stds))
            cmaxes += ",\n               ".join(str(int(float(m)+float(s)*3)) for m, s in zip(means, stds))
            test_descript_file.write(minmax_str % (idx*dt, idx*dt, cmins, cmaxes))
