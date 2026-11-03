# Internal native link driver

This command is invoked by the mobile build pipeline, not by GUI applications.
It uses only Go code and verified precompiled LLVM/target inputs.

For Android, pass its absolute executable path as the Go linker's `-extld`.
Select separate target and host manifests using `GOUI_LINK_INPUTS` and
`GOUI_LINK_TOOLS`. The driver accepts precompiled objects, declared libraries,
and a small explicit set of linker options; it rejects native compilation,
including optional compiler probes requested by Go's linker.

For iOS, use the command as `-extar` and set `GOUI_GO` to the matching Go 1.25.8
executable. The command delegates archive writing to Go pack. Then invoke the
command with `-o Probe go.a host.o ...` to link the independent native host.
The native archive is consumed in full with `-all_load`; it has no symbol index
for general-purpose native archive lookup.

See [the mobile build plan](../../doc/PlanMobileBuild.md#14-第二阶段独立链接组件和-go-链接驱动2026-11-03).
