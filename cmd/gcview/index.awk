BEGIN {print "package main\n\nconst indexHTML = `"; keep = 1}
$1 == "//++" {print substr($0, 6); keep = 1}
/\/\/\--/ {keep = 0}
keep != 0 && $1 != "//++" {print}
END {print "`"}
