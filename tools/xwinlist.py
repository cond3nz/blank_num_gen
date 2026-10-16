#!/usr/bin/env python3
"""Печатает окна X11: имя, геометрию и карту (используется для проверки GUI под wine)."""
import ctypes
import ctypes.util
import sys

xlib = ctypes.CDLL(ctypes.util.find_library("X11") or "libX11.so.6")
xlib.XOpenDisplay.restype = ctypes.c_void_p
xlib.XDefaultRootWindow.restype = ctypes.c_ulong
xlib.XDefaultRootWindow.argtypes = [ctypes.c_void_p]
xlib.XQueryTree.restype = ctypes.c_int
xlib.XQueryTree.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.POINTER(ctypes.c_ulong),
                            ctypes.POINTER(ctypes.c_ulong), ctypes.POINTER(ctypes.POINTER(ctypes.c_ulong)),
                            ctypes.POINTER(ctypes.c_uint)]
ERROR_HANDLER = ctypes.CFUNCTYPE(ctypes.c_int, ctypes.c_void_p, ctypes.c_void_p)
xlib.XSetErrorHandler.restype = ctypes.c_void_p
xlib.XSetErrorHandler.argtypes = [ERROR_HANDLER]


def _ignore_error(display, event):
    return 0


xlib.XSetErrorHandler(ERROR_HANDLER(_ignore_error))
xlib.XFetchName.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.POINTER(ctypes.c_char_p)]
xlib.XGetWindowAttributes.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_void_p]
xlib.XGetGeometry.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.POINTER(ctypes.c_ulong),
                             ctypes.POINTER(ctypes.c_ulong), ctypes.POINTER(ctypes.c_uint),
                             ctypes.POINTER(ctypes.c_uint), ctypes.POINTER(ctypes.c_uint),
                             ctypes.POINTER(ctypes.c_uint)]


class XWindowAttributes(ctypes.Structure):
    _fields_ = [(n, ctypes.c_long) for n in (
        "x", "y", "width", "height", "border_width", "depth", "visual", "root",
        "class", "bit_gravity", "win_gravity", "backing_store", "backing_planes",
        "backing_pixel", "save_under", "colormap", "map_installed", "map_state",
        "all_event_masks", "your_event_mask", "do_not_propagate_mask", "override_redirect",
        "screen")]


def main():
    needle = sys.argv[1] if len(sys.argv) > 1 else ""
    d = xlib.XOpenDisplay(b":0")
    if not d:
        print("нет подключения к X")
        return 1

    root = xlib.XDefaultRootWindow(d)
    found = []

    def walk(win, depth=0):
        r = ctypes.c_ulong()
        parent = ctypes.c_ulong()
        children = ctypes.POINTER(ctypes.c_ulong)()
        count = ctypes.c_uint()
        if not xlib.XQueryTree(d, ctypes.c_ulong(win), ctypes.byref(r), ctypes.byref(parent),
                               ctypes.byref(children), ctypes.byref(count)):
            return
        for i in range(count.value):
            child = children[i]
            attrs = XWindowAttributes()
            xlib.XGetWindowAttributes(d, ctypes.c_ulong(child), ctypes.byref(attrs))
            name = ctypes.c_char_p()
            title = ""
            if attrs.map_state == 2 and xlib.XFetchName(d, ctypes.c_ulong(child), ctypes.byref(name)):
                title = name.value.decode("utf-8", "replace") if name.value else ""
            if title or attrs.map_state == 2:
                print(f"{'  ' * depth}win=0x{child:x} map={attrs.map_state} "
                      f"{attrs.width}x{attrs.height} title={title!r}")
                if needle and needle.lower() in title.lower():
                    found.append((child, title, attrs.map_state, attrs.width, attrs.height))
            walk(child, depth + 1)
            if children:
                xlib.XFree(ctypes.cast(children, ctypes.c_void_p))
            if name:
                pass

    walk(root)
    if needle:
        print("\nСовпадений:", len(found))
        for child, title, state, w, h in found:
            print(f"  {title!r} map_state={state} size={w}x{h}")
    return 0


if __name__ == "__main__":
    sys.exit(main())