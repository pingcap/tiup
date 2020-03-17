/*
 *
 * Copyright 2017 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

#include <errno.h>
#include <sys/eventfd.h>
#include <sys/epoll.h>
#include <unistd.h>
#include <stdio.h>

#ifndef EPOLLEXCLUSIVE
#define EPOLLEXCLUSIVE (1 << 28)
#endif

/* This polling engine is only relevant on linux kernels supporting epoll() */
bool grpc_is_epollexclusive_available(void) {
  static bool logged_why_not = false;

  int fd = epoll_create1(EPOLL_CLOEXEC);
  if (fd < 0) {
    if (!logged_why_not) {
      printf(
              "epoll_create1 failed with error: %d. Not using epollex polling "
              "engine.",
              fd);
      logged_why_not = true;
    }
    return false;
  }
  int evfd = eventfd(0, EFD_NONBLOCK | EFD_CLOEXEC);
  if (evfd < 0) {
    if (!logged_why_not) {
      printf(
              "eventfd failed with error: %d. Not using epollex polling "
              "engine.",
              fd);
      logged_why_not = true;
    }
    close(fd);
    return false;
  }
  struct epoll_event ev;
  /* choose events that should cause an error on
     EPOLLEXCLUSIVE enabled kernels - specifically the combination of
     EPOLLONESHOT and EPOLLEXCLUSIVE */
  ev.events = (uint32_t)(EPOLLET | EPOLLIN | EPOLLEXCLUSIVE | EPOLLONESHOT);
  ev.data.ptr = NULL;
  if (epoll_ctl(fd, EPOLL_CTL_ADD, evfd, &ev) != 0) {
    if (errno != EINVAL) {
      if (!logged_why_not) {
        printf(
            "epoll_ctl with EPOLLEXCLUSIVE | EPOLLONESHOT failed with error: "
            "%d. Not using epollex polling engine.",
            errno);
        logged_why_not = true;
      }
      close(fd);
      close(evfd);
      return false;
    }
  } else {
    if (!logged_why_not) {
      printf(
              "epoll_ctl with EPOLLEXCLUSIVE | EPOLLONESHOT succeeded. This is "
              "evidence of no EPOLLEXCLUSIVE support. Not using "
              "epollex polling engine.");
      logged_why_not = true;
    }
    close(fd);
    close(evfd);
    return false;
  }
  close(evfd);
  close(fd);
  return true;
}

int main() {
    if (grpc_is_epollexclusive_available()) {
        printf("True: epollexclusive is available\n");
        return 0;
    } else {
        printf("False: epollexclusive is NOT available\n");
        return 1;
    }
}
