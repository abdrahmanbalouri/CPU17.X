.name "shooter"
.description "A warrior that shoots far"

fork %:go
ld %0, r2
st r2, 100
st r2, 200
st r2, 300

go:    ld %0, r3
       zjmp %:run

run:   live %1
       st r1, -200
       zjmp %:run