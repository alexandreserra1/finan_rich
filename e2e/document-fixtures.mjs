import { deflateSync } from "node:zlib";

export function xlsxFixture() {
  const files = {
    "[Content_Types].xml": '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>',
    "xl/workbook.xml": '<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Extrato" r:id="rId1"/></sheets></workbook>',
    "xl/_rels/workbook.xml.rels": '<Relationships><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>',
    "xl/worksheets/sheet1.xml": '<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>Data</t></is></c><c r="B1" t="inlineStr"><is><t>Descrição</t></is></c><c r="C1" t="inlineStr"><is><t>Valor</t></is></c></row><row><c r="A2" t="inlineStr"><is><t>15/11/2026</t></is></c><c r="B2" t="inlineStr"><is><t>Mercado da planilha</t></is></c><c r="C2"><v>-45.90</v></c></row></sheetData></worksheet>',
  };
  const local = [];
  const central = [];
  let offset = 0;
  for (const [path, text] of Object.entries(files)) {
    const name = Buffer.from(path);
    const content = Buffer.from(text);
    let checksum = 0xffffffff;
    for (const byte of content) {
      checksum ^= byte;
      for (let bit = 0; bit < 8; bit++) checksum = (checksum >>> 1) ^ ((checksum & 1) ? 0xedb88320 : 0);
    }
    checksum = (checksum ^ 0xffffffff) >>> 0;
    const header = Buffer.alloc(30);
    header.writeUInt32LE(0x04034b50); header.writeUInt16LE(20, 4);
    header.writeUInt32LE(checksum, 14); header.writeUInt32LE(content.length, 18); header.writeUInt32LE(content.length, 22); header.writeUInt16LE(name.length, 26);
    local.push(header, name, content);
    const entry = Buffer.alloc(46);
    entry.writeUInt32LE(0x02014b50); entry.writeUInt16LE(20, 4); entry.writeUInt16LE(20, 6);
    entry.writeUInt32LE(checksum, 16); entry.writeUInt32LE(content.length, 20); entry.writeUInt32LE(content.length, 24); entry.writeUInt16LE(name.length, 28); entry.writeUInt32LE(offset, 42);
    central.push(entry, name);
    offset += header.length + name.length + content.length;
  }
  const directory = Buffer.concat(central);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50); end.writeUInt16LE(Object.keys(files).length, 8); end.writeUInt16LE(Object.keys(files).length, 10); end.writeUInt32LE(directory.length, 12); end.writeUInt32LE(offset, 16);
  return Buffer.concat([...local, directory, end]);
}

export function imagePDF(rgb, width, height, footer = false) {
  const compressed = deflateSync(rgb);
  const stream = Buffer.from(`q\n${width} 0 0 ${height} 0 0 cm\n/Im0 Do\nQ\n${footer ? "BT /F1 10 Tf 30 10 Td (Pagina 1) Tj ET\n" : ""}`);
  const objects = [
    Buffer.from("<< /Type /Catalog /Pages 2 0 R >>"),
    Buffer.from("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
    Buffer.from(`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${width} ${height}] /Resources << /XObject << /Im0 4 0 R >> ${footer ? "/Font << /F1 6 0 R >>" : ""} >> /Contents 5 0 R >>`),
    Buffer.concat([Buffer.from(`<< /Type /XObject /Subtype /Image /Width ${width} /Height ${height} /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length ${compressed.length} >>\nstream\n`), compressed, Buffer.from("\nendstream")]),
    Buffer.concat([Buffer.from(`<< /Length ${stream.length} >>\nstream\n`), stream, Buffer.from("endstream")]),
  ];
  if (footer) objects.push(Buffer.from("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"));
  let pdf = Buffer.from("%PDF-1.4\n");
  const offsets = [0];
  for (const [index, object] of objects.entries()) {
    offsets.push(pdf.length);
    pdf = Buffer.concat([pdf, Buffer.from(`${index + 1} 0 obj\n`), object, Buffer.from("\nendobj\n")]);
  }
  const start = pdf.length;
  const xref = `xref\n0 ${objects.length + 1}\n0000000000 65535 f \n` + offsets.slice(1).map((offset) => String(offset).padStart(10, "0") + " 00000 n \n").join("");
  return Buffer.concat([pdf, Buffer.from(xref + `trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${start}\n%%EOF\n`)]);
}
