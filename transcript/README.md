# ChatGPT transcripts

Please note that this was tested 2026-08-26.

This is likely to fail in future when ChatGPT changes the URL structure.


## Usage

- Create a textfile with the extension `.chatgpt`.
  - Copy and add your prompt
  - Add a blank line
  - Copy and add the URL from ChatGPT
  - Add a blank line
  - and so on
- `wget` all the URL
- run `make`

This will create a `*.md` file from `*.chatgpt`.

