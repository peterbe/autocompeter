# Changelog


## 1.1.7

 * Re-release for minified dist files.

## 1.1.6

 * Ping feature on by default.

## 1.1.5

 * Don't put the hint value behind typed text if it's identical. This
   prevents strangeness when you type longer than the input field such
   that the text becomes right-aligned.

## 1.1.4

 * Ping starts when you put focus on the search widget instead.

## 1.1.3

 * Option to set `{ping: true}` which will pre-flight an AJAX get to the
   server pre-emptively for extra performance. Off by default.

## 1.1.2

 * The Autocompeter doesn't show onload if there is some text in the input
   widget it works on.

## 1.1

 * If the server is slow, the filtering of which results to display is instead
   done using the last result from XHR. This avoids hints from appearing
   that no longer match what you're typing.

## 1.0

 * Inception and the start of maintaining a changelog.
