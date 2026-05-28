.name "shooter"
.description "turns ameba live for us"

        sti r1, %:live, %1
        lfork %2041
        and r1, %0, r1

live:   live %0
        zjmp %:live
