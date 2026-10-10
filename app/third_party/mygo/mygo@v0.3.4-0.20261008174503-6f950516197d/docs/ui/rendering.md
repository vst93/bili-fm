# Rendering

MyGo draws on the GPU with Metal on macOS, with Direct3D 11 on Windows, or
with WARP, Windows' own software renderer, where no GPU driver works, and
with OpenGL on Linux, in the GtkGLArea GTK shows. A shader computes rounded
rectangles, borders, gradients and shadows from the distance to their
edges, so they stay sharp at any size and scale, and text comes from a
glyph atlas that only uploads what changes.

With a GPU, every frame draws on it. On Linux, the first window of
native UI loads OpenGL: Mesa, the driver, takes some 50 MB, which stay,
about as much as the rest of a small app. Where OpenGL would not run on a
GPU, as in virtual machines or in WSL (where `GALLIUM_DRIVER=d3d12` gives
Mesa the GPU), Linux draws on the CPU: about a millisecond for a whole
large window on a high-density display, on several cores, and less than a
tenth of one for what typically changes, such as a button under the
pointer, since it redraws, and has the compositor take, only that. Set
`MYGO_GPU=0` to use the CPU renderer everywhere, for instance to compare,
and on Linux `MYGO_GPU=1` to draw with OpenGL even where it runs on the
CPU.

Set `MYGO_FRAME_STATS=1` to log each frame that takes longer than 8 ms, or
`MYGO_FRAME_STATS=4` for another threshold in milliseconds (`all` logs
every frame). Each line tells how long building the frame (and in how many
passes), laying it out, painting and presenting it took, whether it was
drawn on the GPU or in memory (where a window has no GPU renderer), what
the process allocated and how many texts it laid out
meanwhile, and whether the garbage collector ran, with the time goroutines
spent helping it mark:

```
mygo: frame 412 took 17.3 ms: build 3.1 (2 passes), layout 1.2, paint 0.8, present 11.5 (drawn on the GPU), other 0.7; meanwhile the process made about 2312 allocations, 171.0 KB, 12 text layouts; GC: 1 cycles ended, 4.1 ms of assists
```

Allocations and the collector are the whole process's, other goroutines'
included, as `runtime/metrics` counts them: a span at a time, so that
small counts read low. While the variable is unset, frames measure
nothing.

MyGo draws a frame only when something changes: input, `Invalidate`,
`After`, or an animation that moves. An idle window draws nothing, and two
seconds after its last frame it frees the frame it drew in memory.
