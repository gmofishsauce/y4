ldw r0, r0, 0
ldb r1, r1, 1
stw r3, r3, 0x8
stb r4, r4, 16
beq r3, r3, -20
adi r0, r2, 10
lui r6, 1000
jlr r6, 0, 16
rds lnk, r2
#align 64
lli r5, 42
#align 64
ldi r0, 0xC3C3
