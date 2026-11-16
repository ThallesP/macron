# Ultimate Custom Night, +opengl, the first version of 0009 (no software vertex processing fix)

## trace-0009
| place | draws | glBufferSubData | synchronized maps | unsynchronized writes | unsynchronized maps |
|---|---|---|---|---|---|
| title | 303 | 608 | 0 | 0 | 0 |
| menu | 9726 | 19450 | 0 | 0 | 0 |
| night | 10725 | 21445 | 0 | 1 | 0 |

## trace-control
| place | draws | glBufferSubData | synchronized maps | unsynchronized writes | unsynchronized maps |
|---|---|---|---|---|---|
| title | 341 | 680 | 0 | 0 | 0 |
| menu | 7779 | 15559 | 0 | 0 | 0 |
| night | 7137 | 14268 | 0 | 0 | 0 |

# The complete 0009

## trace2-0009
| place | draws | glBufferSubData | synchronized maps | unsynchronized writes | unsynchronized maps |
|---|---|---|---|---|---|
| title | 292 | 0 | 0 | 584 | 0 |
| menu | 8712 | 0 | 0 | 17423 | 0 |
| night | 9589 | 0 | 0 | 19175 | 0 |

## trace2-control
| place | draws | glBufferSubData | synchronized maps | unsynchronized writes | unsynchronized maps |
|---|---|---|---|---|---|
| title | 236 | 472 | 0 | 0 | 0 |
| menu | 7972 | 15946 | 0 | 0 | 0 |
| night | 9015 | 18028 | 0 | 0 | 0 |

# Frame rates on the software renderer (the sandbox has no GPU OpenGL)
== fps2-old: label=fps2-old run=1 wined3d=m5 wow64cpu=m5 base=apple WINEDEBUG=-all,+fps MACRON_OPENGL=apple-software WINE_D3D_CONFIG=
   start 1: ok, window after 11.8 s
     title, fps median 11.3 mean 11.4 (7.89..14.12, 13 samples), CPU game 89.0% wineserver 2.0% wine-other 2.6% others [[9.4, 'WindowServer'], [7.5, 'storagekitd'], [5.0, 'provjobd3571810459'], [3.5, 'diskarbitrationd']]
     menu , fps median 2.4 mean 2.34 (2.02..2.5, 12 samples), CPU game 101.0% wineserver 0.8% wine-other 2.0% others [[3.6, 'provjobd3571810459'], [3.1, 'WindowServer']]
     night, fps median 1.46 mean 1.42 (1.23..1.51, 11 samples), CPU game 103.0% wineserver 0.8% wine-other 2.2% others [[4.1, 'provjobd3571810459']]
   start 1: ok, window after 14.1 s
     title, fps median 10.86 mean 11.22 (7.38..14.51, 13 samples), CPU game 93.9% wineserver 2.0% wine-other 3.0% others [[10.2, 'WindowServer'], [6.5, 'provjobd3571810459'], [4.2, 'Application']]
     menu , fps median 2.21 mean 2.17 (1.87..2.36, 12 samples), CPU game 102.0% wineserver 0.9% wine-other 2.5% others [[5.0, 'provjobd3571810459'], [3.9, 'WindowServer']]
     night, fps median 1.24 mean 1.26 (1.08..1.39, 11 samples), CPU game 104.0% wineserver 0.9% wine-other 2.7% others [[5.3, 'provjobd3571810459']]
== fps2-0009: label=fps2-0009 run=1 wined3d=m5 wow64cpu=m5 base=apple WINEDEBUG=-all,+fps MACRON_OPENGL=apple-software WINE_D3D_CONFIG=
   start 1: ok, window after 12.6 s
     title, fps median 13.19 mean 13.21 (11.26..14.56, 14 samples), CPU game 94.1% wineserver 1.8% wine-other 2.6% others [[8.4, 'WindowServer'], [5.4, 'Application'], [4.7, 'provjobd3571810459']]
     menu , fps median 2.2 mean 2.13 (1.64..2.48, 12 samples), CPU game 99.1% wineserver 0.9% wine-other 2.4% others [[6.2, 'storagekitd'], [4.6, 'provjobd3571810459'], [3.2, 'WindowServer']]
     night, fps median 1.16 mean 1.19 (0.96..1.44, 10 samples), CPU game 100.1% wineserver 0.9% wine-other 2.6% others [[7.1, 'storagekitd'], [3.4, 'diskarbitrationd'], [3.0, 'provjobd3571810459']]
   start 1: ok, window after 12.8 s
     title, fps median 11.68 mean 10.45 (4.55..14.32, 12 samples), CPU game 85.5% wineserver 2.1% wine-other 3.0% others [[9.7, 'WindowServer'], [7.2, 'provjobd3571810459'], [4.6, 'Application'], [3.9, 'storagekitd']]
     menu , fps median 2.1 mean 2.07 (1.72..2.34, 12 samples), CPU game 101.4% wineserver 0.9% wine-other 2.6% others [[5.7, 'provjobd3571810459'], [3.2, 'WindowServer']]
     night, fps median 1.29 mean 1.3 (1.15..1.43, 10 samples), CPU game 103.9% wineserver 1.0% wine-other 2.7% others []
