/*
Copyright © 2024 Jeff Berkowitz (pdxjjb@gmail.com)

This program is free software: you can redistribute it and/or modify it
under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful, but
WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public
License along with this program. If not, see
<http://www.gnu.org/licenses/>.
*/
package main

//import (
//)

// The opcodes basically spread out to the right, using more and
// more leading 1-bits. The bits come in groups of 3, with the
// special case that 1110... is jlr and 1111... requires decoding
// the next three (XOP) bits. After that, 1111 111... requires
// decoding the next three bits, then 1111 111 111..., etc.
//
// This is implemented by several 8-element function tables where
// the last ("111") element in the table defer to the next table,
// etc. This is the equivalent of chaining 3-to-8 decoders in
// hardware where the "111" (7) output enables the next, then the
// next, etc. This is not really how you'd do it ("ripple" decode,
// too slow) but it's nice and simple here.
func (y4 *y4machine) executeSequential() {
}
