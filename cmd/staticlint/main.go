package main

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/fieldalignment"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"

	"github.com/gostaticanalysis/nilerr"
	"github.com/gostaticanalysis/typednil"
)

// Program staticlint implements a project-specific static analysis multichecker.
//
// The multichecker aggregates several groups of analyzers:
//
//   - стандартные анализаторы пакета golang.org/x/tools/go/analysis/passes:
//     assign, atomic, bools, buildtag, cgocall, composite, copylock,
//     deepequalerrors, errorsas, fieldalignment, httpresponse, ifaceassert,
//     loopclosure, lostcancel, nilfunc, nilness, printf, shadow, shift,
//     sortslice, stdmethods, stringintconv, structtag, testinggoroutine,
//     tests, unmarshal, unreachable, unsafeptr, unusedresult;
//
//   - все анализаторы класса SA пакета staticcheck.io
//     (модуль honnef.co/go/tools/staticcheck);
//
//   - не менее одного анализатора других классов staticcheck.io
//     (в данном проекте используются анализаторы из пакета honnef.co/go/tools/stylecheck);
//
//   - два публичных анализатора из открытых проектов:
//     github.com/gostaticanalysis/nilerr и github.com/gostaticanalysis/typednil;
//
//   - собственный анализатор osExitAnalyzer, который запрещает прямые вызовы
//     os.Exit в функции main пакета main.
//
// Запуск:
//
//	go run ./cmd/staticlint ./...
//
// или после сборки бинарного файла:
//
//	go build -o staticlint ./cmd/staticlint
//	./staticlint ./...
//
// Рекомендуется запускать multichecker перед коммитом изменений, а также в CI,
// чтобы убедиться, что исходный код проходит все подключённые проверки.
func main() {
	var analyzers []*analysis.Analyzer

	// Стандартные анализаторы из golang.org/x/tools/go/analysis/passes.
	analyzers = append(analyzers,
		assign.Analyzer,
		atomic.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		deepequalerrors.Analyzer,
		errorsas.Analyzer,
		fieldalignment.Analyzer,
		httpresponse.Analyzer,
		ifaceassert.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		nilness.Analyzer,
		printf.Analyzer,
		shadow.Analyzer,
		shift.Analyzer,
		sortslice.Analyzer,
		stdmethods.Analyzer,
		stringintconv.Analyzer,
		structtag.Analyzer,
		testinggoroutine.Analyzer,
		tests.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
	)

	// Все анализаторы класса SA пакета staticcheck.io.
	for _, a := range staticcheck.Analyzers {
		if len(a.Analyzer.Name) >= 2 && a.Analyzer.Name[0] == 'S' && a.Analyzer.Name[1] == 'A' {
			analyzers = append(analyzers, a.Analyzer)
		}
	}

	// Не менее одного анализатора остальных классов staticcheck.io.
	// Здесь добавляем все анализаторы из пакета stylecheck (STxxxx).
	for _, a := range stylecheck.Analyzers {
		analyzers = append(analyzers, a.Analyzer)
	}

	// Публичные анализаторы из внешних проектов.
	analyzers = append(analyzers,
		nilerr.Analyzer,
		typednil.Analyzer,
	)

	// Собственный анализатор проекта — запрет прямых os.Exit в main.main.
	analyzers = append(analyzers, osExitAnalyzer)

	multichecker.Main(analyzers...)
}
