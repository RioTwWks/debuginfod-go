package buildid

import (
    "debug/elf"
    "encoding/hex"
    "errors"
    "os"
)

// FromPath извлекает build-id из ELF-файла по пути.
func FromPath(path string) (string, error) {
    f, err := elf.Open(path)
    if err != nil {
        return "", err
    }
    defer f.Close()

    // Ищем секцию с заметками (notes)
    sections := f.Sections
    for _, sec := range sections {
        if sec.Type == elf.SHT_NOTE {
            // Читаем данные секции
            data, err := sec.Data()
            if err != nil {
                continue
            }
            // Парсим заметки вручную или используем стороннюю библиотеку.
            // Здесь можно вызвать функцию из пакета github.com/offlinehacker/buildid[reference:6]
            // или реализовать свой простой парсер.
            // Для MVP можно использовать готовую библиотеку:
            // id, err := buildid.FromPath(path)
            // return id, err
        }
    }
    return "", errors.New("build-id not found")
}
