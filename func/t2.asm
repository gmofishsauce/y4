sys_wrstring = 1

#bank kern_code ; optional

start:
	adi r1, r0, sys_wrstring ; write string syscall
	lui r2, (msg&0xFFC0)>>6
	adi r2, r2, msg&0x3F
	sys
	hlt

#bank kern_data
	#res 0xFFF ; just to make the lui/adi interesting
msg:
	#d "Hello, World!\n"
