
F_APP := cmd/front/
F_OUT := front
TK_APP := cmd/mtask/
TK_OUT := mtask
TM_APP := cmd/mteam/
TM_OUT := mteam


.PHONY: front team task clean
front:	
	go build -o ${F_APP}${F_OUT} ${F_APP}/main.go
	./${F_APP}${F_OUT}

team:
	go build -o ${TM_APP}${TM_OUT} ${TM_APP}/main.go
	./${TM_APP}${TM_OUT}

task:
	go build -o ${TK_APP}${TK_OUT} ${TK_APP}/main.go
	./${TK_APP}${TK_OUT}

clean:
	rm ${F_APP}${F_OUT} ${TK_APP}${TK_OUT} ${TM_APP}${TM_OUT}

	