# TODO list

## Reporting

Structured reporting with filtering. Every call to Report() passes a component,
a phase (build, check, or simulate), a severity, an operation (Clear, Evaluate,
PositiveEdge), an old value (?), a new value, and an optional text message.

## Filtering

The entire report stream is always written to a log as JSON lines (see jsonlines.org).
It is written to the standard output "depending" (maybe command line flags)

## Building

After I write MakeMux, MakeAdder, MakeALU, etc., I will write a simple configuration
language processor to specify the wiring. There will be a single line per component
with fixed fields so it's a regular language (parseable with a state machine).


