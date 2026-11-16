#!/bin/bash
# cmp.sh <clone app> <fresh app>: two game prefixes' file lists and registries, compared.
cd ~/s/compat
a=$1/pfx b=$2/pfx
list() { (cd "$1" && find . \( -type f -o -type l \) -print0 | xargs -0 stat -f '%HT|%z|%N%SY' | perl -pe 's/^Regular File/file/; s/^Symbolic Link/link/'); }
list "$a" > /tmp/la; list "$b" > /tmp/lb
echo "files and links: clone $(wc -l < /tmp/la | tr -d ' '), fresh $(wc -l < /tmp/lb | tr -d ' ')"
echo "== paths in one prefix only, or with another type, size or link target (path: clone | fresh)"
perl -e '
  sub load { my %h; open my $f, "<", shift; while (<$f>) { chomp; my ($t,$s,$p) = split /\|/, $_, 3; $h{$p} = "$t $s"; } %h }
  my %a = load($ARGV[0]); my %b = load($ARGV[1]);
  for my $p (sort keys %{{%a, %b}}) { my ($x,$y) = ($a{$p} // "missing", $b{$p} // "missing"); print "$p: $x | $y\n" if $x ne $y }
' /tmp/la /tmp/lb
same=$(perl -e '
  sub load { my %h; open my $f, "<", shift; while (<$f>) { chomp; my ($t,$s,$p) = split /\|/, $_, 3; $h{$p} = "$t $s"; } %h }
  my %a = load($ARGV[0]); my %b = load($ARGV[1]); my $n = 0; my $c = 0;
  for my $p (keys %a) { next unless exists $b{$p} && $a{$p} eq $b{$p} && $a{$p} =~ /^file/; $n++; }
  print $n' /tmp/la /tmp/lb)
echo "files of the same size in both: $same"
for r in system.reg user.reg userdef.reg; do
  echo "== $r, without time stamps (#time lines and the number after each key)"
  strip() { grep -v -E '^#time=' "$1" | perl -pe 's/^(\[.*\]) \d+$/$1/'; }
  diff <(strip "$a/$r") <(strip "$b/$r")
done
