const fs = require('fs')
const text = fs.readFileSync('r:/BiologicalsUpdate/_dev-scripts-backup/.comparison-cache.json', 'utf8')
const re = /"category":"standard","class":1,"moduleId":(\d+),"name":"([^"]+)","rating":"I","ship":"([^"]+)","symbol":"([^"]+Armour[^"]+)"/g
const newShips = new Set(['Mandalay','Type-11 Prospector','Panther Clipper MkII','Corsair','Caspian Explorer','Kestrel Mk II','Python MkII','Type-8 Transporter','Cobra MkV'])
const seen = new Set()
let m
while ((m = re.exec(text)) !== null) {
  const key = m[4]
  if (newShips.has(m[3]) && !seen.has(key)) {
    seen.add(key)
    console.log(JSON.stringify({id: m[1], symbol: m[4], name: m[2], ship: m[3]}))
  }
}
