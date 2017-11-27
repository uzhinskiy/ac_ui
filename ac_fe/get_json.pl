#!/usr/bin/perl 
use JSON::PP;
require "/usr/local/bin/cust.pl";
my $json = JSON::PP->new;
print $json->encode(\%$customers);
