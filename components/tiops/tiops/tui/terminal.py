# coding: utf-8

from abc import abstractmethod

from .tui import TUI


class TUIDisplay(TUI):
    def __init__(self, inventory):
        super(TUIDisplay, self).__init__()

        self.term = TUI()
        self.inventory = inventory

    # adjust column width to fit the longest content
    def format_columns(self, data):
        result = []
        col_width = []
        for row in data:
            i = 0
            for word in row:
                if not word:
                    continue
                try:
                    col_width[i] = max(len(word), col_width[i])
                except IndexError:
                    col_width.append(len(word))
                i += 1
        for row in data:
            i = 0
            wide_row = []
            for word in row:
                if not word:
                    continue
                wide_row.append(word.ljust(col_width[i] + 2))
                i += 1
            result.append("".join(wide_row))
        return result

    @abstractmethod
    def display(self):
        raise NotImplementedError