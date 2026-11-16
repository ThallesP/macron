#!/usr/bin/env python3
"""Generates dlls/opengl32/batch_table.h for patch 0007 (OpenGL call batching).

Reads Wine's generated dlls/opengl32/unixlib.h (each Unix call's params
struct) and unix_thunks.h (the calls the Unix side wraps) and classifies every
OpenGL call opengl32's Windows side makes:

- queued as it is: returns nothing and has only arguments passed by value;
- queued with the data it points to copied: glBufferSubData, glUniform*v,
  glUniformMatrix*v, glLoadMatrix* and the fixed-size vector calls;
- draws: queued only while no vertex array reads client memory (elements draws
  also need an element array buffer, so their indices are an offset);
- vertex array pointers: queued while a GL_ARRAY_BUFFER is bound (the pointer
  is then an offset), otherwise they mark client arrays in use;
- everything else goes to the Unix side at once, after the queue.

Usage: gl-batch-table.py <wine source>/dlls/opengl32 > batch_table.h
"""
import re
import sys

VALUE_TYPES = {
    "GLenum", "GLboolean", "GLbitfield", "GLbyte", "GLubyte", "GLshort", "GLushort", "GLint", "GLuint",
    "GLsizei", "GLfloat", "GLclampf", "GLdouble", "GLclampd", "GLint64", "GLuint64", "GLint64EXT",
    "GLuint64EXT", "GLintptr", "GLsizeiptr", "GLintptrARB", "GLsizeiptrARB", "GLhalfNV", "GLfixed",
    "GLclampx", "GLhandleARB",
}
TYPE_SIZES = {"b": 1, "ub": 1, "s": 2, "us": 2, "i": 4, "ui": 4, "f": 4, "d": 8, "i64": 8, "ui64": 8}

# Unix-side wrappers that only act on the current context's state, in order:
# safe to run later on the same thread.
SAFE_WRAPPERS = {
    "glClear", "glViewport", "glDrawBuffer", "glReadBuffer", "glFramebufferDrawBufferEXT",
    "glFramebufferReadBufferEXT", "glNamedFramebufferDrawBuffer", "glNamedFramebufferReadBuffer",
}

# Calls that must reach the Unix side when they are made.
NEVER = re.compile(
    r"^gl(Flush|Finish)"          # explicit flushes, mapped range flushes, fences
    r"|^glLockArraysEXT$"         # reads client arrays when called
    r"|^glDebugMessageCallback"   # changes how errors are reported
)


def parse(folder):
    text = open(f"{folder}/unixlib.h").read()
    structs = {}
    for name, body in re.findall(r"^struct (\w+)_params\n\{\n(.*?)^\};", text, re.M | re.S):
        fields = []
        for line in body.strip().splitlines():
            decl = re.sub(r"/\*.*?\*/", "", line).strip().rstrip(";")
            if not (m := re.search(r"(\w+)$", decl)):
                continue  # char message[1]
            field = m.group(1)
            ctype = decl[: -len(field)].strip()
            fields.append((ctype, field))
        structs[name] = fields
    enum = re.search(r"^enum unix_funcs\n\{\n(.*?)^\};", text, re.M | re.S).group(1)
    funcs = [f.strip().rstrip(",")[len("unix_"):] for f in enum.split("\n") if f.strip().startswith("unix_")]
    wrapped = set(re.findall(r"\b(?:wrap|wow64)_(gl\w+)\(", open(f"{folder}/unix_thunks.h").read()))
    return structs, funcs, wrapped


def is_value(ctype):
    return ctype in VALUE_TYPES


def copy_rule(name, pointers, fields):
    """Returns (macro arguments) for calls whose pointed-to data has a size known from the call."""
    if len(pointers) != 1:
        return None
    ctype, field = pointers[0]
    if not ctype.startswith("const ") or ctype.count("*") != 1:
        return None
    names = {f for _, f in fields}
    m = re.match(r"^gl(Named)?BufferSubData(ARB|EXT)?$|^buffer_write_unsynchronized$", name)
    if m and field == "data" and "size" in names:
        return f"BATCH_COPY_N( {name}, data, size, 1 )"
    m = re.match(r"^gl(Program)?Uniform([1-4])(f|i|ui|d|i64|ui64)v(ARB|EXT|NV)?$", name)
    if m and field == "value" and "count" in names:
        return f"BATCH_COPY_N( {name}, value, count, {int(m.group(2)) * TYPE_SIZES[m.group(3)]} )"
    m = re.match(r"^gl(Program)?UniformMatrix([234])(?:x([234]))?(f|d)v(ARB|EXT)?$", name)
    if m and field == "value" and "count" in names:
        rows = int(m.group(2))
        cols = int(m.group(3) or rows)
        return f"BATCH_COPY_N( {name}, value, count, {rows * cols * TYPE_SIZES[m.group(4)]} )"
    m = re.match(r"^gl(Load|Mult)(Transpose)?Matrix(f|d)(ARB)?$", name)
    if m and field == "m":
        return f"BATCH_COPY( {name}, m, {16 * TYPE_SIZES[m.group(3)]} )"
    m = re.match(r"^gl(Color|SecondaryColor|Normal|Vertex|TexCoord|MultiTexCoord|RasterPos|WindowPos|"
                 r"VertexAttrib|VertexAttribI|VertexAttribL)([1-4])N?(b|ub|s|us|i|ui|f|d|i64|ui64)v(ARB|EXT|NV)?$", name)
    if m and field == "v":
        return f"BATCH_COPY( {name}, v, {int(m.group(2)) * TYPE_SIZES[m.group(3)]} )"
    return None


def classify(name, fields, wrapped):
    fields = [f for f in fields if f[1] != "teb"]
    names = [f for _, f in fields]
    pointers = [f for f in fields if not is_value(f[0])]
    flags = []
    if name in ("wglMakeContextCurrentARB", "wglDeleteContext"):
        return "BATCH_FLAGS( %s, BATCH_FORGET_BINDINGS )" % name
    if name.startswith("wgl"):
        return None
    if re.match(r"^glBindBuffer(ARB)?$", name):
        flags.append("BATCH_BIND_BUFFER")
    if re.match(r"^gl(DeleteBuffers|PopClientAttrib)(ARB)?$", name):
        flags.append("BATCH_FORGET_BINDINGS")
    elif re.match(r"^gl(BindVertexArray|DeleteVertexArrays|VertexArrayElementBuffer)(APPLE)?$", name):
        flags.append("BATCH_FORGET_ELEMENTS")
    sets_pointer = ("Pointer" in name or name == "glInterleavedArrays" or "ArrayRange" in name) and not name.startswith("glGet")

    queue = None
    if "ret" in names or NEVER.search(name) or (name in wrapped and name not in SAFE_WRAPPERS):
        queue = None
    elif sets_pointer:
        # an offset into GL_ARRAY_BUFFER, unless no buffer is bound
        if (len(pointers) == 1 and pointers[0] == ("const void *", "pointer")
                and "Element" not in name and "ArrayRange" not in name):
            return "BATCH_POINTER( %s )" % name
        flags.append("BATCH_CLIENT_ARRAYS")
    elif re.match(r"^glDraw(Range)?Elements(BaseVertex|Instanced(ARB|EXT)?(BaseVertex)?(BaseInstance)?|EXT)?$", name):
        if len(pointers) == 1 and pointers[0] == ("const void *", "indices"):
            queue = "BATCH_DRAW_ELEMENTS( %s )" % name
    elif not pointers:
        draw = re.match(r"^gl(Multi)?Draw|^glArrayElement", name)
        queue = ("BATCH_DRAW( %s )" if draw else "BATCH_VALUE( %s )") % name
    else:
        queue = copy_rule(name, pointers, fields)

    if queue and flags:
        assert queue.startswith("BATCH_VALUE("), name
        return "BATCH_VALUE_FLAGS( %s, %s )" % (name, " | ".join(flags))
    if queue:
        return queue
    if flags:
        return "BATCH_FLAGS( %s, %s )" % (name, " | ".join(flags))
    return None


def main():
    structs, funcs, wrapped = parse(sys.argv[1])
    print("/* Generated by Macron's runtime/wine/gl-batch-table.py from unixlib.h and unix_thunks.h; DO NOT EDIT! */")
    print()
    print("static const struct batch_desc batch_descs[funcs_count] =")
    print("{")
    counts = {}
    for name in funcs:
        if name not in structs:
            continue
        entry = classify(name, structs[name], wrapped)
        if entry:
            kind = entry.split("(")[0]
            counts[kind] = counts.get(kind, 0) + 1
            print(f"    {entry},")
    print("};")
    print(" ".join(f"{k.strip()}={v}" for k, v in sorted(counts.items())), file=sys.stderr)


if __name__ == "__main__":
    main()
