.PHONY: all analysis report clean

all: analysis report

analysis:
	python3 analysis.py

report: analysis
	latexmk -pdf II2202-quantitative-exercise.tex

clean:
	latexmk -c II2202-quantitative-exercise.tex
