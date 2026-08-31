# II2202 Research Methodology and Scientific Writing

```bash
cd project-proposal
pdflatex II2202-proposal.tex
pdflatex II2202-proposal.tex
```

With bibliography (after adding `\cite` and `\bibliography{II2202-proposal}`):

```bash
pdflatex II2202-proposal.tex && bibtex II2202-proposal && pdflatex II2202-proposal.tex && pdflatex II2202-proposal.tex
```

Or, if `latexmk` is installed: `latexmk -c -pdf II2202-proposal.tex`

Optional install: `sudo apt install latexmk texlive-luatex`
